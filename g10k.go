package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"github.com/kballard/go-shellquote"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	debug          bool
	verbose        bool
	config         ConfigSettings
	wg             sync.WaitGroup
	empty          struct{}
	syncGitCount   int
	syncForgeCount int
	syncGitTime    float64
	syncForgeTime  float64
)

// ConfigSettings contains the key value pairs from the g10k config file
type ConfigSettings struct {
	CacheDir      string `yaml:"cachedir"`
	ForgeCacheDir string
	Git           struct {
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
}

type GitModule struct {
	git    string
	branch string
	tag    string
	commit string
}

// Debugf is a helper function for debug logging if mainCfgSection["debug"] is set
func Debugf(s string) {
	if debug != false {
		log.Print("DEBUG " + fmt.Sprint(s))
	}
}

// Verbosef is a helper function for debug logging if mainCfgSection["debug"] is set
func Verbosef(s string) {
	if debug != false || verbose != false {
		log.Print(fmt.Sprint(s))
	}
}

// checkDirAndCreate tests if the given directory exists and tries to create it
func checkDirAndCreate(dir string, name string) string {
	if len(dir) != 0 {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			log.Printf("checkDirAndCreate(): trying to create dir '%s'", dir)
			os.Mkdir(dir, 0777)
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
		reGitAttribute := regexp.MustCompile("\\s*:(git|commit|tag|branch)\\s*=>\\s*['\"]?([^'\"]+)['\"]")
		//moduleName := ""
		//nextLineAttr := false

		for _, line := range strings.Split(n, "\n") {
			//fmt.Println(line)
			if strings.Count(line, ":git") > 1 || strings.Count(line, ":tag") > 1 || strings.Count(line, ":branch") > 1 {
				log.Fatal("Error: trailing comma found in ", pf, " somewhere here: ", line)
				os.Exit(1)
			}
			if m := reModuledir.FindStringSubmatch(line); len(m) > 1 {
				puppetFile.moduleDir = m[1]
			} else if m := reForgeModule.FindStringSubmatch(line); len(m) > 1 {
				//fmt.Println("found forge mod name ---> ", m[1])
				if _, ok := puppetFile.forgeModules[m[1]]; ok {
					log.Fatal("Error: Duplicate forge module found in ", pf, " for module ", m[1], " line: ", line)
					os.Exit(1)
				}
				if len(m[3]) > 1 {
					if m[3] == ":latest" {
						puppetFile.forgeModules[m[1]] = ForgeModule{version: "latest"}
					} else {
						puppetFile.forgeModules[m[1]] = ForgeModule{version: m[3]}
					}
					//fmt.Println("found m[1] ---> '", m[1], "'")
					//fmt.Println("found forge mod attribute ---> ", m[3])
				} else {
					//puppetFile.forgeModules[m[1]] = ForgeModule{}
					puppetFile.forgeModules[m[1]] = ForgeModule{version: "latest"}
				}
			} else if m := reGitModule.FindStringSubmatch(line); len(m) > 1 {
				//fmt.Println("found git mod name ---> ", m[1])
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
	//fmt.Println(puppetFile)
	return puppetFile
}

func executeCommand(command string, timeout int) string {
	Debugf("Executing " + command)
	parts := strings.SplitN(command, " ", 2)
	cmd := parts[0]
	cmdArgs := []string{}
	if len(parts) > 1 {
		args, err := shellquote.Split(parts[1])
		if err != nil {
			Debugf("err: " + fmt.Sprint(err))
		} else {
			cmdArgs = args
		}
	}

	before := time.Now()
	out, err := exec.Command(cmd, cmdArgs...).CombinedOutput()
	duration := time.Since(before).Seconds()
	syncGitTime += duration
	Verbosef("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	if err != nil {
		log.Print("git command failed: "+command, err)
		log.Print("Output: " + string(out))
		os.Exit(1)
	}
	return string(out)
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

func doMirrorOrUpdate(url string, workDir string, sshPrivateKey string) {
	dirExists := false
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		dirExists = false
	} else {
		dirExists = true
		//doCheckout = compareGitVersions(workDir, url, branch)
	}

	needSshKey := true
	if strings.Contains(url, "github.com") {
		needSshKey = false
	} else {
		needSshKey = true
		//doCheckout = compareGitVersions(workDir, url, branch)
	}

	gitCmd := "git clone --mirror " + url + " " + workDir
	if dirExists {
		gitCmd = "git --git-dir " + workDir + " remote update"
	}

	if needSshKey {
		executeCommand("ssh-agent bash -c 'ssh-add "+sshPrivateKey+"; "+gitCmd+"'", config.Timeout)
	} else {
		executeCommand(gitCmd, config.Timeout)
	}
}

func doModuleInstallOrNothing(m string) {
	ma := strings.Split(m, "-")
	moduleName := ma[0] + "-" + ma[1]
	moduleVersion := ma[2]
	needToGet := "false"
	workDir := config.ForgeCacheDir + m
	if moduleVersion == "latest" {
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			Debugf("doModuleInstallOrNothing(): " + workDir + " did not exists, fetching module")
			needToGet = "true"
			// check forge API what the latest version is
			needToGet = queryForgeApi(moduleName)
			//fmt.Println(needToGet)
		} else {
			// check forge API if latest version of this module has been updated
			needToGet = queryForgeApi(moduleName)
			//fmt.Println(needToGet)
		}
	}
	if needToGet == "false" {
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			needToGet = "true"
		} else {
			Debugf("doModuleInstallOrNothing(): Using cache for " + moduleName + " in version " + moduleVersion + " because " + workDir + " exists")
		}
	}
	if needToGet != "false" {
		if needToGet != "true" {
			moduleVersion = needToGet
		}

		//fmt.Println("moduleVersion:", moduleVersion)
		//fmt.Println("ma[2]:", ma[2])
		if ma[2] != "latest" {
			createOrPurgeDir(workDir)
		} else {
			if err := os.RemoveAll(workDir); err != nil {
				log.Print("doModuleInstallOrNothing(): error: removing dir failed", err)
			}
			versionDir := strings.Replace(workDir, "latest", moduleVersion, -1)
			if err := os.Symlink(versionDir, workDir); err != nil {
				log.Print("doModuleInstallOrNothing(): Error while trying to symlink ", versionDir, " to ", workDir, " :", err)
				os.Exit(1)
			}
		}

		downloadForgeModule(moduleName, moduleVersion)
	}
}

func queryForgeApi(name string) string {
	//url := "https://forgeapi.puppetlabs.com:443/v3/modules/" + strings.Replace(name, "/", "-", -1)
	url := "https://forgeapi.puppetlabs.com:443/v3/modules?query=" + name
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("Error creating GET request for Puppetlabs forge API", err)
	}
	//if fileInfo, err := os.Stat(cacheDir + "/" + strings.Split(name, "/")[1]); !os.IsNotExist(err) {
	//	req.Header.Set("If-Modified-Since", fileInfo.ModTime().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	//}
	req.Header.Set("User-Agent", "https://github.com/xorpaul/g10k/")
	client := &http.Client{}
	before := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(before).Seconds()
	Verbosef("Querying Forge API " + url + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	syncForgeTime += duration
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
				Debugf("queryForgeApi(): found current version " + strings.Split(m[1], "-")[2])
				return strings.Split(m[1], "-")[2]
			}
		}

		if err != nil {
			panic(err)
		}
		return "false"
	} else if resp.Status == "304 Not Modified" {
		Debugf("doModuleInstallOrNothing(): Got 304 nothing to do for module" + name)
		return "false"
	} else {
		Debugf("doModuleInstallOrNothing(): Unexpected response code" + resp.Status)
		return "false"
	}
	return "false"
}

func downloadForgeModule(name string, version string) {
	//url := "https://forgeapi.puppetlabs.com/v3/files/puppetlabs-apt-2.1.1.tar.gz"
	fileName := name + "-" + version + ".tar.gz"
	if _, err := os.Stat(config.ForgeCacheDir + fileName); os.IsNotExist(err) {
		url := "https://forgeapi.puppetlabs.com/v3/files/" + fileName
		req, err := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "https://github.com/xorpaul/g10k/")
		client := &http.Client{}
		before := time.Now()
		resp, err := client.Do(req)
		duration := time.Since(before).Seconds()
		Verbosef("GETing " + url + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
		syncForgeTime += duration
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		if resp.Status == "200 OK" {
			Debugf("downloadForgeModule(): Trying to create " + config.ForgeCacheDir + fileName)
			out, err := os.Create(config.ForgeCacheDir + fileName)
			if err != nil {
				// panic?
				log.Print("downloadForgeModule(): Error while creating file for Forge module "+config.ForgeCacheDir+fileName, err)
				os.Exit(1)
			}
			defer out.Close()
			io.Copy(out, resp.Body)
			file, err := os.Open(config.ForgeCacheDir + fileName)

			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			defer file.Close()

			var fileReader io.ReadCloser = resp.Body
			if strings.HasSuffix(fileName, ".gz") {
				if fileReader, err = gzip.NewReader(file); err != nil {

					fmt.Println(err)
					os.Exit(1)
				}
				defer fileReader.Close()
			}

			tarBallReader := tar.NewReader(fileReader)
			if err = os.Chdir(config.ForgeCacheDir); err != nil {

				fmt.Println(err)
				os.Exit(1)
			}
			for {
				header, err := tarBallReader.Next()
				if err != nil {
					if err == io.EOF {
						break
					}
					fmt.Println(err)
					os.Exit(1)
				}

				// get the individual filename and extract to the current directory
				filename := header.Name

				switch header.Typeflag {
				case tar.TypeDir:
					// handle directory
					//fmt.Println("Creating directory :", filename)
					err = os.MkdirAll(filename, os.FileMode(header.Mode)) // or use 0755 if you prefer

					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}

				case tar.TypeReg:
					// handle normal file
					//fmt.Println("Untarring :", filename)
					writer, err := os.Create(filename)

					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}

					io.Copy(writer, tarBallReader)

					err = os.Chmod(filename, os.FileMode(header.Mode))

					if err != nil {
						fmt.Println(err)
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

func resolvePuppetEnvironment() {
	allPuppetfiles := make(map[string]Puppetfile)
	for source, sa := range config.Sources {
		wg.Add(1)
		go func(source string, sa Source) {
			defer wg.Done()
			Debugf("Puppet environment: " + source + " (remote=" + sa.Remote + ", basedir=" + sa.Basedir + ", private_key=" + sa.PrivateKey + ", prefix=" + strconv.FormatBool(sa.Prefix) + ")")
			workDir := config.CacheDir + source + ".git"
			// check if sa.Basedir exists
			checkDirAndCreate(sa.Basedir, "basedir")

			//if !strings.Contains(source, "hiera") && !strings.Contains(source, "files") {
			//	gitKey = sa.PrivateKey
			//}
			doMirrorOrUpdate(sa.Remote, workDir, sa.PrivateKey)

			// get all branches
			out := executeCommand("git --git-dir "+workDir+" for-each-ref --sort=-committerdate --format=%(refname:short)", config.Timeout)
			//log.Print(branches)
			branches := strings.Split(out, "\n")
			for _, branch := range branches {
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
							allPuppetfiles[source+"_"+branch] = puppetfile
						}
					}
				}(branch)

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
	uniqueGitModules := make(map[string]string)
	uniqueForgeModules := make(map[string]struct{})
	for env, pf := range allPuppetfiles {
		Debugf("Resolving " + env)
		//fmt.Println(pf)
		for _, gitModule := range pf.gitModules {
			if _, ok := uniqueGitModules[gitModule.git]; !ok {
				uniqueGitModules[gitModule.git] = pf.privateKey
			}
		}
		for forgeModuleName, fm := range pf.forgeModules {
			//fmt.Println("Found Forge module ", forgeModuleName, " with version", fm.version)
			forgeModuleName = strings.Replace(forgeModuleName, "/", "-", -1)
			if _, ok := uniqueForgeModules[forgeModuleName+"-"+fm.version]; !ok {
				uniqueForgeModules[forgeModuleName+"-"+fm.version] = empty
			}
		}
	}
	//fmt.Println(uniqueGitModules)
	resolveGitRepositories(uniqueGitModules)
	resolveForgeModules(uniqueForgeModules)
	//fmt.Println(config.Sources["core"])
	for env, pf := range allPuppetfiles {
		Debugf("Syncing " + env)
		source := strings.Split(env, "_")[0]
		moduleDir := config.Sources[source].Basedir + env + "/" + pf.moduleDir
		createOrPurgeDir(moduleDir)
		for gitName, gitModule := range pf.gitModules {
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
			}
			syncToModuleDir(config.CacheDir+strings.Replace(strings.Replace(gitModule.git, "/", "_", -1), ":", "-", -1), targetDir, tree)
		}
		for forgeModuleName, fm := range pf.forgeModules {
			syncForgeToModuleDir(forgeModuleName, fm, moduleDir)
		}
	}
}

func resolveGitRepositories(uniqueGitModules map[string]string) {
	var wgGit sync.WaitGroup
	for url, sshPrivateKey := range uniqueGitModules {
		wgGit.Add(1)
		go func(url string, sshPrivateKey string) {
			defer wgGit.Done()
			Debugf("git repo url " + url + " with ssh key " + sshPrivateKey)

			// create save directory name from Git repo name
			repoDir := strings.Replace(strings.Replace(url, "/", "_", -1), ":", "-", -1)
			workDir := config.CacheDir + repoDir

			doMirrorOrUpdate(url, workDir, sshPrivateKey)
			//	doCloneOrPull(source, workDir, targetDir, sa.Remote, branch, sa.PrivateKey)

		}(url, sshPrivateKey)
	}
	wgGit.Wait()
}

func createOrPurgeDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		Debugf("createOrPurgeDir(): trying to create dir: " + dir)
		os.Mkdir(dir, 0777)
	} else {
		Debugf("createOrPurgeDir(): Trying to remove: " + dir)
		if err := os.RemoveAll(dir); err != nil {
			log.Print("createOrPurgeDir(): error: removing dir failed", err)
		}
		Debugf("createOrPurgeDir(): trying to create dir: " + dir)
		os.Mkdir(dir, 0777)
	}
}

func syncForgeToModuleDir(name string, m ForgeModule, moduleDir string) {
	syncForgeCount++
	comp := strings.Split(name, "/")
	if len(comp) != 2 {
		log.Print("syncForgeToModuleDir(): forgeModuleName invalid, should be like puppetlabs/apt, but is:", name)
		os.Exit(1)
	} else {
		workDir := config.ForgeCacheDir + strings.Replace(name, "/", "-", -1) + "-" + m.version + "/"
		targetDir := moduleDir + "/" + comp[1]
		Debugf("Trying to symlink " + workDir + " to " + targetDir)
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			log.Print("syncForgeToModuleDir(): Forge module not found in dir: ", workDir)
			os.Exit(1)
		} else {
			if err := os.Symlink(workDir, targetDir); err != nil {
				log.Print("syncForgeToModuleDir(): Error while trying to symlink ", workDir, " to ", targetDir, " :", err)
				os.Exit(1)
			}
		}
	}
}

func syncToModuleDir(srcDir string, targetDir string, tree string) {
	syncGitCount++
	createOrPurgeDir(targetDir)
	cmd := "git --git-dir " + srcDir + " archive " + tree + " | tar -x -C " + targetDir
	before := time.Now()
	_, err := exec.Command("bash", "-c", cmd).Output()
	Verbosef("Executing " + cmd + " took " + strconv.FormatFloat(time.Since(before).Seconds(), 'f', 5, 64) + "s")
	if err != nil {
		log.Printf("Failed to execute command: %s", cmd)
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
		configFile  = flag.String("config", "/home/andpaul/dev/go/src/github.com/xorpaul/g10k/core_envs.yaml", "which config file to use")
		puppetFile  = flag.String("puppetfile", "Puppetfile", "what is the Puppetfile name")
		debugFlag   = flag.Bool("debug", false, "log debug output, defaults to false")
		verboseFlag = flag.Bool("verbose", false, "log verbose output, defaults to false")
	)
	flag.Parse()

	debug = *debugFlag
	verbose = *verboseFlag
	Debugf("Using as config file: " + *configFile)
	Debugf("Using as puppetfile: " + *puppetFile)

	// Limit the number of spare OS threads to the number of logical CPUs on the local machine
	threads := runtime.NumCPU()
	if debug {
		threads = 4
	}

	runtime.GOMAXPROCS(threads)
	config = readConfigfile(*configFile)
	before := time.Now()
	resolvePuppetEnvironment()

	// DEBUG
	//pf := make(map[string]Puppetfile)
	//pf["core_fullmanaged"] = readPuppetfile("/tmp/core/core_fullmanaged/", "/home/andpaul/dev/go/src/github.com/xorpaul/g10k/portal_envs")
	//pf["itodsi_corosync"] = readPuppetfile("/tmp/itodsi/itodsi_corosync/", "/home/andpaul/dev/go/src/github.com/xorpaul/g10k/portal_envs")
	//resolvePuppetfile(pf)
	//resolveGitRepositories(config)
	//resolveForgeModules(configSettings.forge)
	//doModuleInstallOrNothing("camptocamp-postfix-1.2.2", "/tmp/g10k/camptocamp-postfix-1.2.2")
	//doModuleInstallOrNothing("camptocamp-postfix-latest")
	fmt.Println("Synced", syncGitCount, "git repositories and", syncForgeCount, "Forge modules in", strconv.FormatFloat(time.Since(before).Seconds(), 'f', 1, 64), "s with git sync time of", syncGitTime, "s and Forge query + download in", syncForgeTime, "s done in", threads, "threads parallel")
}
