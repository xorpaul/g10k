package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/klauspost/pgzip"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

func doModuleInstallOrNothing(m string, fm ForgeModule) {
	ma := strings.Split(m, "-")
	moduleName := ma[0] + "-" + ma[1]
	moduleVersion := ma[2]
	workDir := config.ForgeCacheDir + m
	fr := ForgeResult{false, ma[2]}
	if check4update {
		moduleVersion = "latest"
	}
	if moduleVersion == "latest" {
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			Debugf("doModuleInstallOrNothing(): " + workDir + " does not exist, fetching module")
			// check forge API what the latest version is
			fr = queryForgeAPI(moduleName, "false", fm)
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
			// XXX: disable adding If-Modified-Since header for now
			// because then the latestForgeModules does not get set with the actual module version for latest
			// maybe if received 304 get the actual version from the -latest symlink
			fr = queryForgeAPI(moduleName, "false", fm)
			//fmt.Println(needToGet)
		}

	} else if moduleVersion == "present" {
		// ensure that a latest version this module exists
		latestDir := config.ForgeCacheDir + moduleName + "-latest"
		if _, err := os.Stat(latestDir); os.IsNotExist(err) {
			if _, ok := uniqueForgeModules[moduleName+"-latest"]; ok {
				Debugf("doModuleInstallOrNothing(): we got " + m + ", but no " + latestDir + " to use, but -latest is already being fetched.")
				return
			}
			Debugf("doModuleInstallOrNothing(): we got " + m + ", but no " + latestDir + " to use. Getting -latest")
			doModuleInstallOrNothing(moduleName+"-latest", fm)
			return
		}
		Debugf("doModuleInstallOrNothing(): Nothing to do for module " + m + ", because " + latestDir + " exists")
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
		downloadForgeModule(moduleName, fr.versionNumber, fm)
	}
}

func queryForgeAPI(name string, file string, fm ForgeModule) ForgeResult {
	//url := "https://forgeapi.puppetlabs.com:443/v3/modules/" + strings.Replace(name, "/", "-", -1)
	baseUrl := config.Forge.Baseurl
	if len(fm.baseUrl) > 0 {
		baseUrl = fm.baseUrl
	}
	url := baseUrl + "/v3/modules?query=" + name
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("queryForgeAPI(): Error creating GET request for Puppetlabs forge API", err)
		os.Exit(1)
	}
	if fileInfo, err := os.Stat(file); err == nil {
		Debugf("queryForgeAPI(): adding If-Modified-Since:" + string(fileInfo.ModTime().Format("Mon, 02 Jan 2006 15:04:05 GMT")) + " to Forge query")
		req.Header.Set("If-Modified-Since", fileInfo.ModTime().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	}
	req.Header.Set("User-Agent", "https://github.com/xorpaul/g10k/")
	req.Header.Set("Connection", "close")

	proxyURL, err := http.ProxyFromEnvironment(req)
	if err != nil {
		Fatalf("queryForgeAPI(): Error while getting http proxy with golang http.ProxyFromEnvironment()" + err.Error())
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
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

		var f interface{}
		if err := json.Unmarshal(body, &f); err != nil {
			Fatalf("queryForgeAPI(): Error while decoding JSON from URL " + url + " Error: " + err.Error())
		}
		currentUri := ""
		m := f.(map[string]interface{})
		for _, v := range m {
			switch vv := v.(type) {
			case []interface{}:
				if len(vv) >= 1 {
					if curRel, ok := vv[0].(map[string]interface{})["current_release"]; ok {
						if val, ok := curRel.(map[string]interface{})["uri"]; ok {
							//fmt.Println("uri --> ", val)
							currentUri = val.(string)
						} else {
							Fatalf("queryForgeAPI(): Error: Unexpected JSON response while trying to figure out what version is current for Forge module " + name + " using " + url)
						}
					} else {
						Fatalf("queryForgeAPI(): Error: Unexpected JSON response while trying to figure out what version is current for Forge module " + name + " using " + url)
					}
				} else {
					Fatalf("queryForgeAPI(): Error: Unexpected JSON response while trying to figure out what version is current for Forge module " + name + " using " + url)
				}
			default:
				// skip, we'll do a sanity for the currentUri value later anyway
			}
		}

		//fmt.Println(string(body))
		if strings.Count(currentUri, "-") < 2 {
			log.Fatal("queryForgeAPI(): Error: Something went wrong while trying to figure out what version is current for Forge module ", name, " ", currentUri, " should contain three '-' characters")
		} else {
			// modified the split because I found a module with version 4.0.0-beta1 mayflower-php
			version := strings.Split(currentUri, name+"-")[1]
			Debugf("queryForgeAPI(): found version " + version + " for " + name + "-latest")
			mutex.Lock()
			latestForgeModules[name] = version
			mutex.Unlock()
			return ForgeResult{true, version}
		}

		if err != nil {
			panic(err)
		}
		return ForgeResult{false, ""}
	} else if resp.Status == "304 Not Modified" {
		Debugf("queryForgeAPI(): Got 304 nothing to do for module " + name)
		return ForgeResult{false, ""}
	} else {
		Debugf("queryForgeAPI(): Unexpected response code " + resp.Status)
		return ForgeResult{false, ""}
	}
}

func downloadForgeModule(name string, version string, fm ForgeModule) {
	//url := "https://forgeapi.puppetlabs.com/v3/files/puppetlabs-apt-2.1.1.tar.gz"
	fileName := name + "-" + version + ".tar.gz"
	if _, err := os.Stat(config.ForgeCacheDir + name + "-" + version); os.IsNotExist(err) {
		baseUrl := config.Forge.Baseurl
		if len(fm.baseUrl) > 0 {
			baseUrl = fm.baseUrl
		}
		url := baseUrl + "/v3/files/" + fileName
		req, err := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "https://github.com/xorpaul/g10k/")
		req.Header.Set("Connection", "close")
		proxyURL, err := http.ProxyFromEnvironment(req)
		if err != nil {
			log.Fatal("downloadForgeModule(): Error while getting http proxy with golang http.ProxyFromEnvironment()", err)
			os.Exit(1)
		}
		client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
		before := time.Now()
		Debugf("GETing " + url)
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

			var fileReader = resp.Body
			if strings.HasSuffix(fileName, ".gz") {
				if fileReader, err = pgzip.NewReader(file); err != nil {

					fmt.Println("downloadForgeModule(): pgzip reader error for module ", fileName, " error:", err)
					os.Exit(1)
				}
				defer fileReader.Close()
			}

			tarBallReader := tar.NewReader(fileReader)
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
				targetFilename := config.ForgeCacheDir + "/" + filename
				//Debugf("downloadForgeModule(): Trying to extract file" + filename)

				switch header.Typeflag {
				case tar.TypeDir:
					// handle directory
					//fmt.Println("Creating directory :", filename)
					//err = os.MkdirAll(targetFilename, os.FileMode(header.Mode)) // or use 0755 if you prefer
					err = os.MkdirAll(targetFilename, os.FileMode(0755)) // or use 0755 if you prefer

					if err != nil {
						fmt.Println("downloadForgeModule(): error while MkdirAll()", filename, err)
						os.Exit(1)
					}

				case tar.TypeReg:
					// handle normal file
					//fmt.Println("Untarring :", filename)
					writer, err := os.Create(targetFilename)

					if err != nil {
						fmt.Println("downloadForgeModule(): error while Create()", filename, err)
						os.Exit(1)
					}

					io.Copy(writer, tarBallReader)

					err = os.Chmod(targetFilename, os.FileMode(0644))

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
	}
	return ForgeModule{name: strings.Split(m["name"].(string), "-")[1], version: m["version"].(string), author: strings.ToLower(m["author"].(string))}
}

func resolveForgeModules(modules map[string]ForgeModule) {
	var wgForge sync.WaitGroup
	for m, fm := range modules {
		wgForge.Add(1)
		go func(m string, fm ForgeModule) {
			defer wgForge.Done()
			Debugf("Trying to get forge module " + m + " with Forge base url " + fm.baseUrl)
			doModuleInstallOrNothing(m, fm)
		}(m, fm)
	}
	wgForge.Wait()
}

func check4ForgeUpdate(moduleName string, currentVersion string, latestVersion string) {
	Verbosef("found currently deployed Forge module " + moduleName + " in version: " + currentVersion)
	Verbosef("found latest Forge module of " + moduleName + " in version: " + latestVersion)
	if currentVersion != latestVersion {
		color.Yellow("ATTENTION: Forge module: " + moduleName + " latest: " + latestVersion + " currently deployed: " + currentVersion)
		needSyncForgeCount++
	}
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
			if check4update {
				me := readModuleMetadata(targetDir + "metadata.json")
				check4ForgeUpdate(m.name, me.version, latestForgeModules[moduleName])
			}
			return
		}
		// safe to do, because we ensured in doModuleInstallOrNothing() that -latest exists
		m.version = "latest"

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
		if check4update {
			check4ForgeUpdate(m.name, me.version, latestForgeModules[moduleName])
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
		Fatalf("syncForgeToModuleDir(): Forge module not found in dir: " + workDir)
	} else {
		Infof("Need to sync " + targetDir)
		cmd := "cp --link --archive " + workDir + "* " + targetDir
		if usemove {
			cmd = "mv " + workDir + "* " + targetDir
		}
		mutex.Lock()
		needSyncForgeCount++
		mutex.Unlock()
		if !dryRun {
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
}
