package main

import (
	"archive/tar"
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/klauspost/pgzip"
	"github.com/tidwall/gjson"
)

func doModuleInstallOrNothing(m string, fm ForgeModule) {
	ma := strings.Split(m, "-")
	moduleName := ma[0] + "-" + ma[1]
	moduleVersion := ma[2]
	workDir := config.ForgeCacheDir + m
	fr := ForgeResult{false, ma[2], "", 0}
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
					absolutePath, err := filepath.Abs(versionDir)
					Debugf("doModuleInstallOrNothing(): trying to create symlink " + workDir + " pointing to " + absolutePath)
					if err != nil {
						Fatalf("doModuleInstallOrNothing(): Error while resolving absolute file path for " + versionDir + " Error: " + err.Error())
					}
					if err := os.Symlink(absolutePath, workDir); err != nil {
						Fatalf("doModuleInstallOrNothing(): 1 Error while creating symlink " + workDir + " pointing to " + absolutePath + err.Error())
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

	//log.Println("fr.needToGet for ", m, fr.needToGet)

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
				absolutePath, err := filepath.Abs(versionDir)
				if err != nil {
					Fatalf("doModuleInstallOrNothing(): Error while resolving absolute file path for " + versionDir + " Error: " + err.Error())
				}
				Debugf("doModuleInstallOrNothing(): trying to create symlink " + workDir + " pointing to " + absolutePath)
				if err := os.Symlink(absolutePath, workDir); err != nil {
					Fatalf("doModuleInstallOrNothing(): 2 Error while creating symlink " + workDir + " pointing to " + absolutePath + err.Error())
				}
			}
		}
		downloadForgeModule(moduleName, fr.versionNumber, fm, 1)
	}

}

func queryForgeAPI(name string, file string, fm ForgeModule) ForgeResult {
	//url := "https://forgeapi.puppetlabs.com:443/v3/modules/" + strings.Replace(name, "/", "-", -1)
	baseUrl := config.Forge.Baseurl
	if len(fm.baseUrl) > 0 {
		baseUrl = fm.baseUrl
	}
	//url := baseUrl + "/v3/modules?query=" + name
	url := baseUrl + "/v3/releases?module=" + name + "&owner=" + fm.author + "&sort_by=release_date&limit=1"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		Fatalf("queryForgeAPI(): Error creating GET request for Puppetlabs forge API" + err.Error())
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
	if err != nil {
		Fatalf("queryForgeAPI(): Error while issuing the HTTP request to " + url + " Error: " + err.Error())
	}
	duration := time.Since(before).Seconds()
	Verbosef("Querying Forge API " + url + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")

	mutex.Lock()
	syncForgeTime += duration
	mutex.Unlock()
	defer resp.Body.Close()

	if resp.Status == "200 OK" {
		// need to get latest version
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			Fatalf("queryForgeAPI(): Error while reading response body for Forge module " + fm.name + " from " + url + ": " + err.Error())
		}

		before := time.Now()
		currentRelease := gjson.Get(string(body), "results.0").Map()

		duration := time.Since(before).Seconds()
		version := currentRelease["version"].String()
		moduleHashsum := currentRelease["file_md5"].String()
		moduleFilesize := currentRelease["file_size"].Int()

		mutex.Lock()
		forgeJsonParseTime += duration
		mutex.Unlock()

		Debugf("queryForgeAPI(): found version " + version + " for " + name + "-latest")
		mutex.Lock()
		latestForgeModules[name] = version
		mutex.Unlock()
		return ForgeResult{true, version, moduleHashsum, moduleFilesize}

	} else if resp.Status == "304 Not Modified" {
		Debugf("queryForgeAPI(): Got 304 nothing to do for module " + name)
		return ForgeResult{false, "", "", 0}
	} else {
		Debugf("queryForgeAPI(): Unexpected response code " + resp.Status)
		return ForgeResult{false, "", "", 0}
	}
}

// getMetadataForgeModule queries the configured Puppet Forge and return
func getMetadataForgeModule(fm ForgeModule) ForgeModule {
	baseUrl := config.Forge.Baseurl
	if len(fm.baseUrl) > 0 {
		baseUrl = fm.baseUrl
	}
	url := baseUrl + "/v3/releases/" + fm.author + "-" + fm.name + "-" + fm.version
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "https://github.com/xorpaul/g10k/")
	req.Header.Set("Connection", "close")
	proxyURL, err := http.ProxyFromEnvironment(req)
	if err != nil {
		Fatalf("getMetadataForgeModule(): Error while getting http proxy with golang http.ProxyFromEnvironment()" + err.Error())
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	before := time.Now()
	Debugf("GETing " + url)
	resp, err := client.Do(req)
	duration := time.Since(before).Seconds()
	Verbosef("GETing Forge metadata from " + url + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	mutex.Lock()
	syncForgeTime += duration
	mutex.Unlock()
	if err != nil {
		Fatalf("getMetadataForgeModule(): Error while querying metadata for Forge module " + fm.name + " from " + url + ": " + err.Error())
	}
	defer resp.Body.Close()

	if resp.Status == "200 OK" {
		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			Fatalf("getMetadataForgeModule(): Error while reading response body for Forge module " + fm.name + " from " + url + ": " + err.Error())
		}

		before := time.Now()
		currentRelease := gjson.Parse(string(body)).Map()
		duration := time.Since(before).Seconds()
		moduleHashsum := currentRelease["file_md5"].String()
		moduleFilesize := currentRelease["file_size"].Int()
		Debugf("getMetadataForgeModule: module: " + fm.author + "/" + fm.name + " moduleHashsum: " + moduleHashsum + " moduleFilesize: " + strconv.FormatInt(moduleFilesize, 10))

		mutex.Lock()
		forgeJsonParseTime += duration
		mutex.Unlock()

		return ForgeModule{hashSum: moduleHashsum, fileSize: moduleFilesize}
	} else {
		Fatalf("getMetadataForgeModule(): Unexpected response code while GETing " + url + resp.Status)
	}
	return ForgeModule{}
}

func downloadForgeModule(name string, version string, fm ForgeModule, retryCount int) {
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
			Fatalf("downloadForgeModule(): Error while getting http proxy with golang http.ProxyFromEnvironment()" + err.Error())
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
			Fatalf("downloadForgeModule(): Error while GETing Forge module " + name + " from " + url + ": " + err.Error())
		}
		defer resp.Body.Close()

		if resp.Status == "200 OK" {
			before = time.Now()
			Debugf("downloadForgeModule(): Trying to create " + config.ForgeCacheDir + fileName)
			out, err := os.Create(config.ForgeCacheDir + fileName)
			if err != nil {
				Fatalf("downloadForgeModule(): Error while creating file for Forge module " + config.ForgeCacheDir + fileName + " Error: " + err.Error())
			}
			defer out.Close()
			io.Copy(out, resp.Body)
			file, err := os.Open(config.ForgeCacheDir + fileName)

			if err != nil {
				Fatalf("downloadForgeModule(): Error while opening file " + config.ForgeCacheDir + fileName + " Error: " + err.Error())
			}

			defer file.Close()

			var fileReader = resp.Body
			if strings.HasSuffix(fileName, ".gz") {
				if fileReader, err = pgzip.NewReader(file); err != nil {
					Fatalf("downloadForgeModule(): pgzip reader error for module " + fileName + " error:" + err.Error())
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
					Fatalf("downloadForgeModule(): error while tar reader.Next() for " + fileName + err.Error())
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
						Fatalf("downloadForgeModule(): error while MkdirAll()" + filename + err.Error())
					}

				case tar.TypeReg:
					// handle normal file
					//fmt.Println("Untarring :", filename)
					writer, err := os.Create(targetFilename)

					if err != nil {
						Fatalf("downloadForgeModule(): error while Create()" + filename + err.Error())
					}

					io.Copy(writer, tarBallReader)

					err = os.Chmod(targetFilename, os.FileMode(0644))

					if err != nil {
						Fatalf("downloadForgeModule(): error while Chmod()" + filename + err.Error())
					}

					writer.Close()
				default:
					Fatalf("downloadForgeModule(): Unable to untar type: " + string(header.Typeflag) + " in file " + filename)
				}
			}

			duration = time.Since(before).Seconds()
			Verbosef("Extracting " + url + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
			mutex.Lock()
			ioForgeTime += duration
			mutex.Unlock()
		} else {
			Fatalf("downloadForgeModule(): Unexpected response code while GETing " + url + resp.Status)
		}
	} else {
		Debugf("downloadForgeModule(): Using cache for Forge module " + name + " version: " + version)
	}

	if checkSum {
		fm.version = version
		if doForgeModuleIntegrityCheck(fm) {
			if retryCount == 0 {
				Fatalf("downloadForgeModule(): giving up for Puppet module " + name + " version: " + version)
			}
			Warnf("Retrying...")
			purgeDir(config.ForgeCacheDir+fileName, "downloadForgeModule()")
			purgeDir(strings.Replace(config.ForgeCacheDir+fileName, ".tar.gz", "/", -1), "downloadForgeModule()")
			// retry if hash sum mismatch found
			downloadForgeModule(name, version, fm, retryCount-1)
		}
	}

}

// readModuleMetadata returns the Forgemodule struct of the given module file path
func readModuleMetadata(file string) ForgeModule {
	content, _ := ioutil.ReadFile(file)

	before := time.Now()
	name := gjson.Get(string(content), "name").String()
	version := gjson.Get(string(content), "version").String()
	author := gjson.Get(string(content), "author").String()
	duration := time.Since(before).Seconds()
	mutex.Lock()
	metadataJsonParseTime += duration
	mutex.Unlock()

	Debugf("readModuleMetadata(): Found in file " + file + " name: " + name + " version: " + version + " author: " + author)

	moduleName := "N/A"
	if strings.Contains(name, "-") {
		moduleName = strings.Split(name, "-")[1]
	} else {
		Debugf("readModuleMetadata(): Error: Something went wrong while decoding file " + file + " searching for the module name (found for name: " + name + "), version and author")
	}

	return ForgeModule{name: moduleName, version: version, author: strings.ToLower(author)}
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

func doForgeModuleIntegrityCheck(m ForgeModule) bool {
	var wgCheckSum sync.WaitGroup

	wgCheckSum.Add(1)
	fmm := ForgeModule{}
	go func(m ForgeModule) {
		defer wgCheckSum.Done()
		fmm = getMetadataForgeModule(m)
		Debugf("doForgeModuleChecksumCheck(): target md5 hash sum: " + fmm.hashSum)
	}(m)

	wgCheckSum.Add(1)
	calculatedHashSum := "N/A"
	var calculatedArchiveSize int64
	fileName := config.ForgeCacheDir + m.author + "-" + m.name + "-" + m.version + ".tar.gz"
	go func(m ForgeModule) {
		defer wgCheckSum.Done()

		before := time.Now()
		if fi, err := os.Stat(fileName); err == nil {
			calculatedArchiveSize = fi.Size()
			file, err := os.Open(fileName)
			if err != nil {
				Fatalf("doForgeModuleChecksumCheck(): Can't access Forge module archive " + fileName + " ! Error: " + err.Error())
			}
			defer file.Close()

			Debugf("doForgeModuleChecksumCheck(): Trying to get md5 check sum for " + fileName)
			hash := md5.New()
			if _, err := io.Copy(hash, file); err != nil {
				Fatalf("doForgeModuleChecksumCheck(): Error while reading Forge module archive " + fileName + " ! Error: " + err.Error())
			}

			calculatedHashSum = hex.EncodeToString(hash.Sum(nil))
			Debugf("doForgeModuleChecksumCheck(): calculated md5 hash sum: " + calculatedHashSum)

		} else {
			Fatalf("doForgeModuleChecksumCheck(): Can't access Forge module archive " + fileName + " ! Error: " + err.Error())
		}
		duration := time.Since(before).Seconds()
		Debugf("Calculating hash sum for " + fileName + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
		Debugf("doForgeModuleChecksumCheck(): calculated archive size: " + strconv.FormatInt(calculatedArchiveSize, 10))
	}(m)

	wgCheckSum.Wait()

	if fmm.hashSum != calculatedHashSum {
		Warnf("WARNING: calculated md5sum " + calculatedHashSum + " for " + fileName + " does not match expected md5sum " + fmm.hashSum)
		return true
	} else {
		Debugf("OK: calculated md5sum " + calculatedHashSum + " for " + fileName + " does match expected md5sum " + fmm.hashSum)
		if fmm.fileSize != calculatedArchiveSize {
			Warnf("WARNING: calculated file size " + strconv.FormatInt(calculatedArchiveSize, 10) + " for " + fileName + " does not match expected file size " + strconv.FormatInt(fmm.fileSize, 10))
			return true
		}
		Debugf("OK: calculated file size " + strconv.FormatInt(calculatedArchiveSize, 10) + " for " + fileName + " does match expected file size " + strconv.FormatInt(fmm.fileSize, 10))
	}
	return false

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
			ioForgeTime += duration
			mutex.Unlock()
			Verbosef("Executing " + cmd + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
			if err != nil {
				Fatalf("syncForgeToModuleDir(): Failed to execute command: " + cmd + " Output: " + string(out) + "\nError while trying to hardlink " + workDir + " to " + targetDir + " :" + err.Error())
			}
		}
	}
}
