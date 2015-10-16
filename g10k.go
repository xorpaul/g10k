package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/kballard/go-shellquote"
	"github.com/klauspost/pgzip"
	"gopkg.in/yaml.v2"
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
	"syscall"
	"time"
)

var (
	debug              bool
	verbose            bool
	info               bool
	force              bool
	config             ConfigSettings
	wg                 sync.WaitGroup
	mutex              sync.Mutex
	empty              struct{}
	syncGitCount       int
	syncForgeCount     int
	syncGitTime        float64
	syncForgeTime      float64
	cpGitTime          float64
	cpForgeTime        float64
	buildtime          string
	uniqueForgeModules map[string]struct{}
	latestForgeModules map[string]string
)

// ConfigSettings contains the key value pairs from the g10k config file
type ConfigSettings struct {
	CacheDir        string `yaml:"cachedir"`
	ForgeCacheDir   string
	ModulesCacheDir string
	EnvCacheDir     string
	Git             struct {
		privateKey string `yaml:"private_key"`
		username   string
	}
	Sources map[string]Source
	Timeout int `yaml:"timeout"`
}

type Source struct {
	Remote     string
	Basedir    string
	Prefix     bool
	PrivateKey string `yaml:"private_key"`
}

// Puppetfile contains the key value pairs from the Puppetfile
type Puppetfile struct {
	moduleDir    string
	forgeModules map[string]ForgeModule
	gitModules   map[string]GitModule
	privateKey   string
}

type ForgeModule struct {
	version string
	name    string
	author  string
}

type GitModule struct {
	git    string
	branch string
	tag    string
	commit string
	ref    string
}

type ForgeResult struct {
	needToGet     bool
	versionNumber string
}

type ExecResult struct {
	returnCode int
	output     string
}

// Debugf is a helper function for debug logging if global variable debug is set to true
func Debugf(s string) {
	if debug != false {
		log.Print("DEBUG " + fmt.Sprint(s))
	}
}

// Verbosef is a helper function for debug logging if global variable verbose is set to true
func Verbosef(s string) {
	if debug != false || verbose != false {
		log.Print(fmt.Sprint(s))
	}
}

// Infof is a helper function for debug logging if global variable info is set to true
func Infof(s string) {
	if debug != false || verbose != false || info != false {
		fmt.Println(s)
	}
}

// fileExists checks if the given file exists and return a bool
func fileExists(file string) bool {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

// checkDirAndCreate tests if the given directory exists and tries to create it
func checkDirAndCreate(dir string, name string) string {
	if len(dir) != 0 {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			//log.Printf("checkDirAndCreate(): trying to create dir '%s' as %s", dir, name)
			if err := os.MkdirAll(dir, 0777); err != nil {
				log.Print("checkDirAndCreate(): Error: failed to create directory: ", dir)
				os.Exit(1)
			}
		}
	} else {
		// TODO make dir optional
		log.Print("dir setting '" + name + "' missing! Exiting!")
		os.Exit(1)
	}
	if !strings.HasSuffix(dir, "/") {
		dir = dir + "/"
	}
	Debugf("Using as " + name + ": " + dir)
	return dir
}

// readConfigfile creates the ConfigSettings struct from the g10k config file
func readConfigfile(configFile string) ConfigSettings {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Print("There was an error parsing the config file "+configFile+": ", err)
		os.Exit(1)
	}

	//fmt.Println("data:", string(data))
	data = bytes.Replace(data, []byte(":cachedir:"), []byte("cachedir:"), -1)
	//fmt.Println("data:", string(data))
	var config ConfigSettings
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("YAML unmarshal error: %v", err)
	}

	//fmt.Println("config:", config)
	//for k, v := range config.Sources {
	//	log.Print(k)
	//	log.Print(v.Remote)
	//}

	// check if cachedir exists
	config.CacheDir = checkDirAndCreate(config.CacheDir, "cachedir")
	config.ForgeCacheDir = checkDirAndCreate(config.CacheDir+"forge/", "cachedir/forge")
	config.ModulesCacheDir = checkDirAndCreate(config.CacheDir+"modules/", "cachedir/modules")
	config.EnvCacheDir = checkDirAndCreate(config.CacheDir+"environments/", "cachedir/environments")

	// set default timeout to 5 seconds if no timeout setting found
	if config.Timeout == 0 {
		config.Timeout = 5
	}

	return config
}

// readPuppetfile creates the ConfigSettings struct from the Puppetfile
func readPuppetfile(targetDir string, sshKey string) Puppetfile {
	var puppetFile Puppetfile
	puppetFile.privateKey = sshKey
	puppetFile.forgeModules = map[string]ForgeModule{}
	puppetFile.gitModules = map[string]GitModule{}
	pf := targetDir + "Puppetfile"
	if _, err := os.Stat(pf); os.IsNotExist(err) {
		Debugf("readPuppetfile(): No Puppetfile found in " + targetDir)
	} else {
		Debugf("readPuppetfile(): Trying to parse: " + pf)
		file, err := os.Open(pf)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		reComment := regexp.MustCompile("\\s*#")
		reEmpty := regexp.MustCompile("^$")

		n := ""
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if !reComment.MatchString(line) && !reEmpty.MatchString(line) {
				if regexp.MustCompile(",\\s*$").MatchString(line) {
					n += line
				} else {
					n += line + "\n"
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		reModuledir := regexp.MustCompile("^\\s*(?:moduledir)\\s*['\"]?([^'\"]+)['\"]")
		reForgeModule := regexp.MustCompile("^\\s*(?:mod)\\s*['\"]?([^'\"]+/[^'\"]+)['\"](?:\\s*(,)\\s*['\"]?([^'\"]*))?")
		reGitModule := regexp.MustCompile("^\\s*(?:mod)\\s*['\"]?([^'\"/]+)['\"]\\s*,(.*)")
		reGitAttribute := regexp.MustCompile("\\s*:(git|commit|tag|branch|ref)\\s*=>\\s*['\"]?([^'\"]+)['\"]")
		//moduleName := ""
		//nextLineAttr := false

		for _, line := range strings.Split(n, "\n") {
			//fmt.Println(line)
			if strings.Count(line, ":git") > 1 || strings.Count(line, ":tag") > 1 || strings.Count(line, ":branch") > 1 || strings.Count(line, ":ref") > 1 {
				log.Fatal("Error: trailing comma found in ", pf, " somewhere here: ", line)
				os.Exit(1)
			}
			if m := reModuledir.FindStringSubmatch(line); len(m) > 1 {
				puppetFile.moduleDir = m[1]
			} else if m := reForgeModule.FindStringSubmatch(line); len(m) > 1 {
				//fmt.Println("found forge mod name ---> ", m[1])
				comp := strings.Split(m[1], "/")
				if len(comp) != 2 {
					log.Print("Forge module name is invalid, should be like puppetlabs/apt, but is:", m[3], " line: ", line)
					os.Exit(1)
				}
				if _, ok := puppetFile.forgeModules[m[1]]; ok {
					log.Fatal("Error: Duplicate forge module found in ", pf, " for module ", m[1], " line: ", line)
					os.Exit(1)
				}
				if len(m[3]) > 1 {
					if m[3] == ":latest" {
						puppetFile.forgeModules[m[1]] = ForgeModule{version: "latest", name: comp[1], author: comp[0]}
					} else {
						puppetFile.forgeModules[m[1]] = ForgeModule{version: m[3], name: comp[1], author: comp[0]}
					}
					//fmt.Println("found m[1] ---> '", m[1], "'")
					//fmt.Println("found forge mod attribute ---> ", m[3])
				} else {
					//puppetFile.forgeModules[m[1]] = ForgeModule{}
					puppetFile.forgeModules[m[1]] = ForgeModule{version: "present", name: comp[1], author: comp[0]}
				}
			} else if m := reGitModule.FindStringSubmatch(line); len(m) > 1 {
				//fmt.Println("found git mod name ---> ", m[1])
				if strings.Contains(m[1], "-") {
					fmt.Println("Warning: Found invalid character '-' in Puppet module name", m[1], " line:", line)
				}
				if len(m[2]) > 1 {
					gitModuleAttributes := m[2]
					if strings.Count(gitModuleAttributes, ":git") < 1 {
						log.Fatal("Error: Missing :git url in ", pf, " for module ", m[1], " line: ", line)
						os.Exit(1)
					}
					if strings.Count(gitModuleAttributes, ",") > 1 {
						log.Fatal("Error: Too many attributes in ", pf, " for module ", m[1], " line: ", line)
						os.Exit(1)
					}
					if _, ok := puppetFile.gitModules[m[1]]; ok {
						log.Fatal("Error: Duplicate module found in ", pf, " for module ", m[1], " line: ", line)
						os.Exit(1)
					}
					puppetFile.gitModules[m[1]] = GitModule{}
					//fmt.Println("found git mod attribute ---> ", gitModuleAttributes)
					if a := reGitAttribute.FindStringSubmatch(gitModuleAttributes); len(a) > 1 {
						gm := GitModule{}
						//fmt.Println("found for git mod ", m[1], " attribute ", a[1], " with value ", a[2])
						if a[1] == "git" {
							gm.git = a[2]
						} else if a[1] == "branch" {
							gm.branch = a[2]
						} else if a[1] == "tag" {
							gm.tag = a[2]
						} else if a[1] == "commit" {
							gm.commit = a[2]
						} else if a[1] == "ref" {
							gm.ref = a[2]
						}
						if strings.Contains(gitModuleAttributes, ",") {
							if a := reGitAttribute.FindStringSubmatch(strings.SplitN(gitModuleAttributes, ",", 2)[1]); len(a) > 1 {
								if a[1] == "git" {
									gm.git = a[2]
								} else if a[1] == "branch" {
									gm.branch = a[2]
								} else if a[1] == "tag" {
									gm.tag = a[2]
								} else if a[1] == "commit" {
									gm.commit = a[2]
								} else if a[1] == "ref" {
									gm.ref = a[2]
								}
								//puppetFile.gitModules[m[1]] = GitModule{a[1]: a[2]}
								//fmt.Println("found for git mod ", m[1], " attribute ", a[1], " with value ", a[2])
							}

						}
						puppetFile.gitModules[m[1]] = gm
					}
				}
			}
		}

	}
	// check if we need to set defaults
	if len(puppetFile.moduleDir) == 0 {
		puppetFile.moduleDir = "modules"
	}
	//fmt.Println(puppetFile)
	return puppetFile
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

func executeCommand(command string, timeout int, allowFail bool) ExecResult {
	Debugf("Executing " + command)
	parts := strings.SplitN(command, " ", 2)
	cmd := parts[0]
	cmdArgs := []string{}
	if len(parts) > 1 {
		args, err := shellquote.Split(parts[1])
		if err != nil {
			Debugf("executeCommand(): err: " + fmt.Sprint(err))
		} else {
			cmdArgs = args
		}
	}

	before := time.Now()
	out, err := exec.Command(cmd, cmdArgs...).CombinedOutput()
	duration := time.Since(before).Seconds()
	er := ExecResult{0, string(out)}
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		er.returnCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}
	mutex.Lock()
	syncGitTime += duration
	mutex.Unlock()
	Verbosef("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	if err != nil && !allowFail {
		log.Print("executeCommand(): git command failed: "+command, err)
		log.Print("executeCommand(): Output: " + string(out))
		os.Exit(1)
	}
	return er
}

//func compareGitVersions(targetDir string, url string, branch string) bool {
//	localChan := make(chan string)
//	remoteChan := make(chan string)
//
//	go func() {
//		localOut := executeCommand("git --git-dir "+targetDir++"/.git rev-parse HEAD")
//		localVersion := string(localOut[:len(localOut)-1])
//		Debugf("git output: " + localVersion)
//		Debugf("localVersion: " + localVersion)
//		localChan <- localVersion
//	}()
//
//	go func() {
//		remoteArgs := []string{}
//		remoteArgs = append(remoteArgs, "ls-remote")
//		remoteArgs = append(remoteArgs, "--heads")
//		remoteArgs = append(remoteArgs, url)
//		remoteArgs = append(remoteArgs, branch)
//
//		remoteVersion := executeCommand(remoteArgs, "")
//		Debugf("git output: " + remoteVersion)
//
//		remoteLine := strings.Split(string(remoteVersion), "\t")
//		if remoteLine != nil && len(remoteLine) > 0 {
//			remoteVersion = remoteLine[0]
//		}
//
//		Debugf("remoteVersion: " + remoteVersion)
//		remoteChan <- remoteVersion
//	}()
//	return <-remoteChan != <-localChan
//}

func doMirrorOrUpdate(url string, workDir string, sshPrivateKey string, allowFail bool) bool {
	dirExists := false
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		dirExists = false
	} else {
		dirExists = true
		//doCheckout = compareGitVersions(workDir, url, branch)
	}

	needSshKey := true
	if strings.Contains(url, "github.com") || len(sshPrivateKey) == 0 {
		needSshKey = false
	} else {
		needSshKey = true
		//doCheckout = compareGitVersions(workDir, url, branch)
	}

	er := ExecResult{}
	gitCmd := "git clone --mirror " + url + " " + workDir
	if dirExists {
		gitCmd = "git --git-dir " + workDir + " remote update"
	}

	if needSshKey {
		er = executeCommand("ssh-agent bash -c 'ssh-add "+sshPrivateKey+"; "+gitCmd+"'", config.Timeout, allowFail)
	} else {
		er = executeCommand(gitCmd, config.Timeout, allowFail)
	}

	if er.returnCode != 0 {
		fmt.Println("WARN: git repository " + url + " does not exist or is unreachable at this moment!")
		return false
	}
	return true
}

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

func resolvePuppetEnvironment(envBranch string) {
	allPuppetfiles := make(map[string]Puppetfile)
	for source, sa := range config.Sources {
		wg.Add(1)
		go func(source string, sa Source) {
			defer wg.Done()
			if force {
				createOrPurgeDir(sa.Basedir, "resolvePuppetEnvironment()")
			}
			sa.Basedir = checkDirAndCreate(sa.Basedir, "basedir for source "+source)
			Debugf("Puppet environment: " + source + " (remote=" + sa.Remote + ", basedir=" + sa.Basedir + ", private_key=" + sa.PrivateKey + ", prefix=" + strconv.FormatBool(sa.Prefix) + ")")
			if len(sa.PrivateKey) > 0 {
				if _, err := os.Stat(sa.PrivateKey); err != nil {
					log.Println("resolvePuppetEnvironment(): could not find SSH private key ", sa.PrivateKey, "error: ", err)
					os.Exit(1)
				}
			}
			//if _, err := os.Stat(sa.Basedir); os.IsNotExist(err) {
			//	log.Println("resolvePuppetEnvironment(): could not access ", sa.Basedir)
			//	os.Exit(1)
			//}
			workDir := config.EnvCacheDir + source + ".git"
			// check if sa.Basedir exists
			checkDirAndCreate(sa.Basedir, "basedir")

			//if !strings.Contains(source, "hiera") && !strings.Contains(source, "files") {
			//	gitKey = sa.PrivateKey
			//}
			if success := doMirrorOrUpdate(sa.Remote, workDir, sa.PrivateKey, true); success {

				// get all branches
				er := executeCommand("git --git-dir "+workDir+" for-each-ref --sort=-committerdate --format=%(refname:short)", config.Timeout, false)
				branches := strings.Split(strings.TrimSpace(er.output), "\n")

				for _, branch := range branches {
					if len(envBranch) > 0 && branch != envBranch {
						Debugf("Skipping branch " + branch)
						continue
					}
					wg.Add(1)

					go func(branch string) {
						defer wg.Done()
						if len(branch) != 0 {
							Debugf("Resolving branch: " + branch)
							// TODO if sa.Prefix != true
							targetDir := sa.Basedir + source + "_" + branch + "/"
							syncToModuleDir(workDir, targetDir, branch)
							if _, err := os.Stat(targetDir + "Puppetfile"); os.IsNotExist(err) {
								Debugf("Skipping branch " + source + "_" + branch + " because " + targetDir + "Puppetfile does not exitst")
							} else {
								puppetfile := readPuppetfile(targetDir, sa.PrivateKey)
								mutex.Lock()
								allPuppetfiles[source+"_"+branch] = puppetfile
								mutex.Unlock()

							}
						}
					}(branch)

				}
			}
		}(source, sa)
	}

	wg.Wait()
	//fmt.Println("allPuppetfiles: ", allPuppetfiles, len(allPuppetfiles))
	//fmt.Println("allPuppetfiles[0]: ", allPuppetfiles["postinstall"])
	resolvePuppetfile(allPuppetfiles)
	//// sync to basedir
	//for _, branch := range branches {
	//	if len(branch) != 0 {
	//		Debugf("Syncing branch: " + branch)
	//		// TODO if sa.Prefix != true
	//		if !strings.Contains(branch, "hiera") && !strings.Contains(branch, "files") {
	//			//puppetfile := readPuppetfile(targetDir)

	//		}
	//	}
	//}
}

func resolvePuppetfile(allPuppetfiles map[string]Puppetfile) {
	var wg sync.WaitGroup
	uniqueGitModules := make(map[string]string)
	uniqueForgeModules = make(map[string]struct{})
	latestForgeModules = make(map[string]string)
	exisitingModuleDirs := make(map[string]struct{})
	for env, pf := range allPuppetfiles {
		Debugf("Resolving " + env)
		//fmt.Println(pf)
		for _, gitModule := range pf.gitModules {
			mutex.Lock()
			if _, ok := uniqueGitModules[gitModule.git]; !ok {
				uniqueGitModules[gitModule.git] = pf.privateKey
			}
			mutex.Unlock()
		}
		for forgeModuleName, fm := range pf.forgeModules {
			//fmt.Println("Found Forge module ", forgeModuleName, " with version", fm.version)
			mutex.Lock()
			forgeModuleName = strings.Replace(forgeModuleName, "/", "-", -1)
			if _, ok := uniqueForgeModules[forgeModuleName+"-"+fm.version]; !ok {
				uniqueForgeModules[forgeModuleName+"-"+fm.version] = empty
			}
			mutex.Unlock()
		}
	}
	//fmt.Println(uniqueGitModules)
	resolveGitRepositories(uniqueGitModules)
	resolveForgeModules(uniqueForgeModules)
	//fmt.Println(config.Sources["core"])
	for env, pf := range allPuppetfiles {
		Debugf("Syncing " + env)
		source := strings.Split(env, "_")[0]
		basedir := checkDirAndCreate(config.Sources[source].Basedir, "basedir for source "+source)
		moduleDir := basedir + env + "/" + pf.moduleDir
		if force {
			createOrPurgeDir(moduleDir, "resolvePuppetfile()")
			moduleDir = checkDirAndCreate(moduleDir, "moduleDir for source "+source)
		} else {
			moduleDir = checkDirAndCreate(moduleDir, "moduleDir for "+source)
			exisitingModuleDirsFI, _ := ioutil.ReadDir(moduleDir)
			mutex.Lock()
			for _, exisitingModuleDir := range exisitingModuleDirsFI {
				exisitingModuleDirs[moduleDir+exisitingModuleDir.Name()] = empty
			}
			mutex.Unlock()
		}
		for gitName, gitModule := range pf.gitModules {
			wg.Add(1)
			go func(gitName string, gitModule GitModule) {
				defer wg.Done()
				//fmt.Println(gitModule)
				//fmt.Println("source: " + source)
				targetDir := moduleDir + "/" + gitName
				//fmt.Println("targetDir: " + targetDir)
				tree := "master"
				if len(gitModule.branch) > 0 {
					tree = gitModule.branch
				} else if len(gitModule.commit) > 0 {
					tree = gitModule.commit
				} else if len(gitModule.tag) > 0 {
					tree = gitModule.tag
				} else if len(gitModule.ref) > 0 {
					tree = gitModule.ref
				}
				syncToModuleDir(config.ModulesCacheDir+strings.Replace(strings.Replace(gitModule.git, "/", "_", -1), ":", "-", -1), targetDir, tree)

				// remove this module from the exisitingModuleDirs map
				mutex.Lock()
				if _, ok := exisitingModuleDirs[moduleDir+gitName]; ok {
					delete(exisitingModuleDirs, moduleDir+gitName)
				}
				mutex.Unlock()
			}(gitName, gitModule)
		}
		for forgeModuleName, fm := range pf.forgeModules {
			wg.Add(1)
			go func(forgeModuleName string, fm ForgeModule) {
				defer wg.Done()
				syncForgeToModuleDir(forgeModuleName, fm, moduleDir)
				// remove this module from the exisitingModuleDirs map
				mutex.Lock()
				if _, ok := exisitingModuleDirs[moduleDir+fm.name]; ok {
					delete(exisitingModuleDirs, moduleDir+fm.name)
				}
				mutex.Unlock()
			}(forgeModuleName, fm)
		}
	}
	wg.Wait()
	//fmt.Println(uniqueForgeModules)
	if len(exisitingModuleDirs) > 0 {
		for d, _ := range exisitingModuleDirs {
			Debugf("resolvePuppetfile(): Removing unmanaged file " + d)
			if err := os.RemoveAll(d); err != nil {
				Debugf("resolvePuppetfile(): Error while trying to remove unmanaged file " + d)
			}
		}
	}
}

func resolveGitRepositories(uniqueGitModules map[string]string) {
	var wgGit sync.WaitGroup
	for url, sshPrivateKey := range uniqueGitModules {
		wgGit.Add(1)
		go func(url string, sshPrivateKey string) {
			defer wgGit.Done()
			if len(sshPrivateKey) > 0 {
				Debugf("git repo url " + url + " with ssh key " + sshPrivateKey)
			} else {
				Debugf("git repo url " + url + " without ssh key")
			}

			// create save directory name from Git repo name
			repoDir := strings.Replace(strings.Replace(url, "/", "_", -1), ":", "-", -1)
			workDir := config.ModulesCacheDir + repoDir

			doMirrorOrUpdate(url, workDir, sshPrivateKey, false)
			//	doCloneOrPull(source, workDir, targetDir, sa.Remote, branch, sa.PrivateKey)

		}(url, sshPrivateKey)
	}
	wgGit.Wait()
}

func createOrPurgeDir(dir string, callingFunction string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		Debugf("createOrPurgeDir(): Trying to create dir: " + dir + " called from " + callingFunction)
		os.Mkdir(dir, 0777)
	} else {
		Debugf("createOrPurgeDir(): Trying to remove: " + dir + " called from " + callingFunction)
		if err := os.RemoveAll(dir); err != nil {
			log.Print("createOrPurgeDir(): error: removing dir failed", err)
		}
		Debugf("createOrPurgeDir(): Trying to create dir: " + dir + " called from " + callingFunction)
		os.Mkdir(dir, 0777)
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

func syncToModuleDir(srcDir string, targetDir string, tree string) {
	mutex.Lock()
	syncGitCount++
	mutex.Unlock()
	logCmd := "git --git-dir " + srcDir + " log -n1 --pretty=format:%H " + tree
	er := executeCommand(logCmd, config.Timeout, false)
	hashFile := targetDir + "/.latest_commit"
	needToSync := true
	if len(er.output) > 0 {
		targetHash, _ := ioutil.ReadFile(hashFile)
		if string(targetHash) == er.output {
			needToSync = false
			//Debugf("syncToModuleDir(): Skipping, because no diff found between " + srcDir + "(" + er.output + ") and " + targetDir + "(" + string(targetHash) + ")")
		}

	}
	if needToSync {
		Infof("Need to sync " + targetDir)
		createOrPurgeDir(targetDir, "syncToModuleDir()")
		cmd := "git --git-dir " + srcDir + " archive " + tree + " | tar -x -C " + targetDir
		before := time.Now()
		out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		duration := time.Since(before).Seconds()
		mutex.Lock()
		cpGitTime += duration
		mutex.Unlock()
		Verbosef("syncToModuleDir(): Executing " + cmd + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
		if err != nil {
			log.Println("syncToModuleDir(): Failed to execute command: ", cmd, " Output: ", string(out))
			os.Exit(1)
		}

		er = executeCommand(logCmd, config.Timeout, false)
		if len(er.output) > 0 {
			Debugf("Writing hash " + er.output + " from command " + logCmd + " to " + hashFile)
			f, _ := os.Create(hashFile)
			defer f.Close()
			f.WriteString(er.output)
			f.Sync()
		}
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

func main() {

	var (
		configFile    = flag.String("config", "", "which config file to use")
		envBranchFlag = flag.String("branch", "", "which git branch of the Puppet environment to update, e.g. core_foobar")
		forceFlag     = flag.Bool("force", false, "purge the Puppet environment directory and do a full sync")
		debugFlag     = flag.Bool("debug", false, "log debug output, defaults to false")
		verboseFlag   = flag.Bool("verbose", false, "log verbose output, defaults to false")
		infoFlag      = flag.Bool("info", false, "log info output, defaults to false")
		versionFlag   = flag.Bool("version", false, "show build time and version number")
	)
	flag.Parse()

	debug = *debugFlag
	verbose = *verboseFlag
	info = *infoFlag
	force = *forceFlag

	if *versionFlag {
		fmt.Println("g10k Version 1.0 Build time:", buildtime, "UTC")
		os.Exit(0)
	}

	if len(os.Getenv("VIMRUNTIME")) > 0 {
		*configFile = "/home/andpaul/dev/go/src/github.com/xorpaul/g10k/test.yaml"
		*envBranchFlag = "invalid_modulename"
		debug = true
	}

	if len(*configFile) > 0 {
		Debugf("Using as config file: " + *configFile)
	} else {
		log.Println("Error: no config file set")
		log.Printf("Example call: %s -config test.yaml\n", os.Args[0])
		os.Exit(1)
	}

	config = readConfigfile(*configFile)
	before := time.Now()
	envText := *configFile
	if len(*envBranchFlag) > 0 {
		resolvePuppetEnvironment(*envBranchFlag)
		envText += " with branch " + *envBranchFlag
	} else {
		resolvePuppetEnvironment("")
	}

	// DEBUG
	//pf := make(map[string]Puppetfile)
	//pf["core_fullmanaged"] = readPuppetfile("/tmp/core/core_fullmanaged/", "/home/andpaul/dev/go/src/github.com/xorpaul/g10k/portal_envs")
	//pf["itodsi_corosync"] = readPuppetfile("/tmp/itodsi/itodsi_corosync/", "/home/andpaul/dev/go/src/github.com/xorpaul/g10k/portal_envs")
	//resolvePuppetfile(pf)
	//resolveGitRepositories(config)
	//resolveForgeModules(configSettings.forge)
	//doModuleInstallOrNothing("camptocamp-postfix-1.2.2", "/tmp/g10k/camptocamp-postfix-1.2.2")
	//doModuleInstallOrNothing("saz-resolv_conf-latest")
	//readModuleMetadata("/tmp/g10k/forge/camptocamp-postfix-1.2.2/metadata.json")

	fmt.Println("Synced", envText, "with", syncGitCount, "git repositories and", syncForgeCount, "Forge modules in", strconv.FormatFloat(time.Since(before).Seconds(), 'f', 1, 64), "s with git (", strconv.FormatFloat(syncGitTime, 'f', 1, 64), "s sync, I/O", strconv.FormatFloat(cpGitTime, 'f', 1, 64), "s) and Forge (", strconv.FormatFloat(syncForgeTime, 'f', 1, 64), "s query+download, I/O", strconv.FormatFloat(cpForgeTime, 'f', 1, 64), "s)")
}
