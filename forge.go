package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"github.com/klauspost/pgzip"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func doModuleInstallOrNothing(m string) {
	ma := strings.Split(m, "-")
	moduleName := ma[0] + "-" + ma[1]
	moduleVersion := ma[2]
	workDir := config.ForgeCacheDir + m
	fr := ForgeResult{false, ma[2]}
	if moduleVersion == "latest" {
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			Debugf("doModuleInstallOrNothing(): " + workDir + " did not exists, fetching module")
			// check forge API what the latest version is
			fr = queryForgeApi(moduleName, "false")
			if fr.needToGet {
				if _, ok := uniqueForgeModules[moduleName+"-"+fr.versionNumber]; ok {
					Debugf("doModuleInstallOrNothing(): no need to fetch Forge module " + moduleName + " in latest, because latest is " + fr.versionNumber + " and that will already be fetched")
					fr.needToGet = false
					versionDir := config.ForgeCacheDir + moduleName + "-" + fr.versionNumber
					Debugf("doModuleInstallOrNothing(): trying to create symlink " + workDir + " pointing to " + versionDir)
					if err := os.Symlink(versionDir, workDir); err != nil {
						log.Println("doModuleInstallOrNothing(): 1 Error while create symlink "+workDir+" pointing to "+versionDir, err)
						os.Exit(1)
					}
					//} else {
					//Debugf("doModuleInstallOrNothing(): need to fetch Forge module " + moduleName + " in latest, because version " + fr.versionNumber + " will not be fetched already")

					//fmt.Println(needToGet)
				}
			}
		} else {
			// check forge API if latest version of this module has been updated
			Debugf("doModuleInstallOrNothing(): check forge API if latest version of module " + moduleName + " has been updated")
			// XXX: disable adding If-Modified-Since head for now
			// because then the latestForgeModules does not get set with the actual module version for latest
			// maybe if received 304 get the actual version from the -latest symlink
			fr = queryForgeApi(moduleName, "false")
			//fmt.Println(needToGet)
		}

	} else if moduleVersion == "present" {
		// ensure that a latest version this module exists
		latestDir := config.ForgeCacheDir + moduleName + "-latest"
		if _, err := os.Stat(latestDir); os.IsNotExist(err) {
			if _, ok := uniqueForgeModules[moduleName+"-latest"]; ok {
				Debugf("doModuleInstallOrNothing(): we got " + m + ", but no " + latestDir + " to use, but -latest is already being fetched.")
				return
			} else {
				Debugf("doModuleInstallOrNothing(): we got " + m + ", but no " + latestDir + " to use. Getting -latest")
				doModuleInstallOrNothing(moduleName + "-latest")
			}
			return
		} else {
			Debugf("doModuleInstallOrNothing(): Nothing to do for module " + m + ", because " + latestDir + " exists")
		}
	} else {
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			fr.needToGet = true
		} else {
			Debugf("doModuleInstallOrNothing(): Using cache for " + moduleName + " in version " + moduleVersion + " because " + workDir + " exists")
			return
		}
	}

	//fmt.Println("fr.needToGet for ", m, fr.needToGet)

	if fr.needToGet {
		if ma[2] != "latest" {
			Debugf("doModuleInstallOrNothing(): Trying to remove: " + workDir)
			_ = os.Remove(workDir)
		} else {
			versionDir, _ := os.Readlink(workDir)
			if versionDir == config.ForgeCacheDir+moduleName+"-"+fr.versionNumber {
				Debugf("doModuleInstallOrNothing(): No reason to re-symlink again")
			} else {
				Debugf("doModuleInstallOrNothing(): Trying to remove symlink: " + workDir)
				_ = os.Remove(workDir)
				versionDir = config.ForgeCacheDir + moduleName + "-" + fr.versionNumber
				Debugf("doModuleInstallOrNothing(): trying to create symlink " + workDir + " pointing to " + versionDir)
				if err := os.Symlink(versionDir, workDir); err != nil {
					log.Println("doModuleInstallOrNothing(): 2 Error while create symlink "+workDir+" pointing to "+versionDir, err)
					os.Exit(1)
				}
			}
		}
		downloadForgeModule(moduleName, fr.versionNumber)
	}
}

func queryForgeApi(name string, file string) ForgeResult {
	//url := "https://forgeapi.puppetlabs.com:443/v3/modules/" + strings.Replace(name, "/", "-", -1)
	url := "https://forgeapi.puppetlabs.com:443/v3/modules?query=" + name
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("queryForgeApi(): Error creating GET request for Puppetlabs forge API", err)
		os.Exit(1)
	}
	if fileInfo, err := os.Stat(file); err == nil {
		Debugf("queryForgeApi(): adding If-Modified-Since:" + string(fileInfo.ModTime().Format("Mon, 02 Jan 2006 15:04:05 GMT")) + " to Forge query")
		req.Header.Set("If-Modified-Since", fileInfo.ModTime().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	}
	req.Header.Set("User-Agent", "https://github.com/xorpaul/g10k/")
	req.Header.Set("Connection", "close")

	proxyUrl, err := http.ProxyFromEnvironment(req)
	if err != nil {
		log.Fatal("queryForgeApi(): Error while getting http proxy with golang http.ProxyFromEnvironment()", err)
		os.Exit(1)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}
	before := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(before).Seconds()
	Verbosef("Querying Forge API " + url + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	mutex.Lock()
	syncForgeTime += duration
	mutex.Unlock()
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.Status == "200 OK" {
		// need to get latest version
		body, err := ioutil.ReadAll(resp.Body)

		//fmt.Println(string(body))
		reCurrent := regexp.MustCompile("\\s*\"current_release\": {\n\\s*\"uri\": \"([^\"]+)\",")
		if m := reCurrent.FindStringSubmatch(string(body)); len(m) > 1 {
			//fmt.Println(m[1])
			if strings.Count(m[1], "-") < 2 {
				log.Fatal("queryForgeApi(): Error: Something went wrong while trying to figure out what version is current for Forge module ", name, " ", m[1], " should contain three '-' characters")
				os.Exit(1)
			} else {
				version := strings.Split(m[1], "-")[2]
				Debugf("queryForgeApi(): found version " + version + " for " + name + "-latest")
				mutex.Lock()
				latestForgeModules[name] = version
				mutex.Unlock()
				return ForgeResult{true, version}
			}
		}

		if err != nil {
			panic(err)
		}
		return ForgeResult{false, ""}
	} else if resp.Status == "304 Not Modified" {
		Debugf("queryForgeApi(): Got 304 nothing to do for module " + name)
		return ForgeResult{false, ""}
	} else {
		Debugf("queryForgeApi(): Unexpected response code " + resp.Status)
		return ForgeResult{false, ""}
	}
	return ForgeResult{false, ""}
}

func downloadForgeModule(name string, version string) {
	//url := "https://forgeapi.puppetlabs.com/v3/files/puppetlabs-apt-2.1.1.tar.gz"
	fileName := name + "-" + version + ".tar.gz"
	if _, err := os.Stat(config.ForgeCacheDir + name + "-" + version); os.IsNotExist(err) {
		url := "https://forgeapi.puppetlabs.com/v3/files/" + fileName
		req, err := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "https://github.com/xorpaul/g10k/")
		req.Header.Set("Connection", "close")
		proxyUrl, err := http.ProxyFromEnvironment(req)
		if err != nil {
			log.Fatal("downloadForgeModule(): Error while getting http proxy with golang http.ProxyFromEnvironment()", err)
			os.Exit(1)
		}
		client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}
		before := time.Now()
		resp, err := client.Do(req)
		duration := time.Since(before).Seconds()
		Verbosef("GETing " + url + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
		mutex.Lock()
		syncForgeTime += duration
		mutex.Unlock()
		if err != nil {
			log.Print("downloadForgeModule(): Error while GETing Forge module ", name, " from ", url, ": ", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.Status == "200 OK" {
			Debugf("downloadForgeModule(): Trying to create " + config.ForgeCacheDir + fileName)
			out, err := os.Create(config.ForgeCacheDir + fileName)
			if err != nil {
				log.Print("downloadForgeModule(): Error while creating file for Forge module "+config.ForgeCacheDir+fileName, err)
				os.Exit(1)
			}
			defer out.Close()
			io.Copy(out, resp.Body)
			file, err := os.Open(config.ForgeCacheDir + fileName)

			if err != nil {
				fmt.Println("downloadForgeModule(): Error while opening file", file, err)
				os.Exit(1)
			}

			defer file.Close()

			var fileReader io.ReadCloser = resp.Body
			if strings.HasSuffix(fileName, ".gz") {
				if fileReader, err = pgzip.NewReader(file); err != nil {

					fmt.Println("downloadForgeModule(): pgzip reader error for module ", fileName, " error:", err)
					os.Exit(1)
				}
				defer fileReader.Close()
			}

			tarBallReader := tar.NewReader(fileReader)
			if err = os.Chdir(config.ForgeCacheDir); err != nil {

				fmt.Println("downloadForgeModule(): error while chdir to", config.ForgeCacheDir, err)
				os.Exit(1)
			}
			for {
				header, err := tarBallReader.Next()
				if err != nil {
					if err == io.EOF {
						break
					}
					fmt.Println("downloadForgeModule(): error while tar reader.Next() for ", fileName, err)
					os.Exit(1)
				}

				// get the individual filename and extract to the current directory
				filename := header.Name
				//Debugf("downloadForgeModule(): Trying to extract file" + filename)

				switch header.Typeflag {
				case tar.TypeDir:
					// handle directory
					//fmt.Println("Creating directory :", filename)
					//err = os.MkdirAll(filename, os.FileMode(header.Mode)) // or use 0755 if you prefer
					err = os.MkdirAll(filename, os.FileMode(0755)) // or use 0755 if you prefer

					if err != nil {
						fmt.Println("downloadForgeModule(): error while MkdirAll()", filename, err)
						os.Exit(1)
					}

				case tar.TypeReg:
					// handle normal file
					//fmt.Println("Untarring :", filename)
					writer, err := os.Create(filename)

					if err != nil {
						fmt.Println("downloadForgeModule(): error while Create()", filename, err)
						os.Exit(1)
					}

					io.Copy(writer, tarBallReader)

					err = os.Chmod(filename, os.FileMode(0644))

					if err != nil {
						fmt.Println("downloadForgeModule(): error while Chmod()", filename, err)
						os.Exit(1)
					}

					writer.Close()
				default:
					fmt.Printf("Unable to untar type : %c in file %s", header.Typeflag, filename)
				}
			}

		} else {
			log.Print("downloadForgeModule(): Unexpected response code while GETing " + url + resp.Status)
			os.Exit(1)
		}
	} else {
		Debugf("downloadForgeModule(): Using cache for Forge module " + name + " version: " + version)
	}
}

// readModuleMetadata returns the Forgemodule struct of the given module file path
func readModuleMetadata(file string) ForgeModule {
	content, _ := ioutil.ReadFile(file)
	var f interface{}
	if err := json.Unmarshal(content, &f); err != nil {
		Debugf("readModuleMetadata(): err: " + fmt.Sprint(err))
		return ForgeModule{}
	}
	m := f.(map[string]interface{})
	if !strings.Contains(m["name"].(string), "-") {
		return ForgeModule{}
	} else {
		return ForgeModule{name: strings.Split(m["name"].(string), "-")[1], version: m["version"].(string), author: strings.ToLower(m["author"].(string))}
	}
}

func resolveForgeModules(modules map[string]struct{}) {
	var wgForge sync.WaitGroup
	for m := range modules {
		wgForge.Add(1)
		go func(m string) {
			defer wgForge.Done()
			Debugf("Trying to get forge module " + m)
			doModuleInstallOrNothing(m)
		}(m)
	}
	wgForge.Wait()
}

func syncForgeToModuleDir(name string, m ForgeModule, moduleDir string) {
	mutex.Lock()
	syncForgeCount++
	mutex.Unlock()
	moduleName := strings.Replace(name, "/", "-", -1)
	//Debugf("syncForgeToModuleDir(): m.name " + m.name + " m.version " + m.version + " moduleName " + moduleName)
	targetDir := moduleDir + m.name
	targetDir = checkDirAndCreate(targetDir, "as targetDir for module "+name)
	if m.version == "present" {
		if _, err := os.Stat(targetDir + "metadata.json"); err == nil {
			Debugf("syncForgeToModuleDir(): Nothing to do, found existing Forge module: " + targetDir + "metadata.json")
			return
		} else {
			// safe to do, because we ensured in doModuleInstallOrNothing() that -latest exists
			m.version = "latest"
		}

	}
	if _, err := os.Stat(targetDir + "metadata.json"); err == nil {
		me := readModuleMetadata(targetDir + "metadata.json")
		if m.version == "latest" {
			//log.Println(latestForgeModules)
			if _, ok := latestForgeModules[moduleName]; ok {
				Debugf("syncForgeToModuleDir(): using version " + latestForgeModules[moduleName] + " for " + moduleName + "-" + m.version)
				m.version = latestForgeModules[moduleName]
			}
		}
		if me.version == m.version {
			Debugf("syncForgeToModuleDir(): Nothing to do, existing Forge module: " + targetDir + " has the same version " + me.version + " as the to be synced version: " + m.version)
			return
		}
		log.Println("syncForgeToModuleDir(): Need to sync, because existing Forge module: " + targetDir + " has version " + me.version + " and the to be synced version is: " + m.version)
		createOrPurgeDir(targetDir, " targetDir for module "+me.name)
	}
	workDir := config.ForgeCacheDir + moduleName + "-" + m.version + "/"
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		log.Print("syncForgeToModuleDir(): Forge module not found in dir: ", workDir)
		os.Exit(1)
	} else {
		Infof("Need to sync " + targetDir)
		cmd := "cp --link --archive " + workDir + "* " + targetDir
		before := time.Now()
		out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		duration := time.Since(before).Seconds()
		mutex.Lock()
		cpForgeTime += duration
		mutex.Unlock()
		Verbosef("Executing " + cmd + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
		if err != nil {
			log.Println("Failed to execute command: ", cmd, " Output: ", string(out))
			log.Print("syncForgeToModuleDir(): Error while trying to hardlink ", workDir, " to ", targetDir, " :", err)
			os.Exit(1)
		}
	}
}
