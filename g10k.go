package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/kballard/go-shellquote"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	debug              bool
	verbose            bool
	config             ConfigSettings
	wg                 sync.WaitGroup
	wgGit              sync.WaitGroup
	uniqueGitModules   map[string]struct{}
	uniqueGitModulesMU sync.Mutex
	empty              struct{}
)

// ConfigSettings contains the key value pairs from the g10k config file
type ConfigSettings struct {
	CacheDir string `yaml:"cachedir"`
	Git      struct {
		privateKey string `yaml:"private_key"`
		username   string
	}
	Sources map[string]Source
}

type Source struct {
	Remote     string
	Basedir    string
	Prefix     bool
	PrivateKey string `yaml:"private_key"`
}

// PuppetfileSettings contains the key value pairs from the Puppetfile
type PuppetfileSettings struct {
	moduleDir    string
	forgeModules map[string]ForgeModule
	gitModules   map[string]GitModule
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
			log.Printf("trying to create dir '%s'", dir)
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
		log.Fatalf("error: %v", err)
	}
	//fmt.Println("config:", config)
	//for k, v := range config.Sources {
	//	log.Print(k)
	//	log.Print(v.Remote)
	//}

	// check if cachedir exists
	config.CacheDir = checkDirAndCreate(config.CacheDir, "cachedir")

	return config
}

// readPuppetfile creates the ConfigSettings struct from the Puppetfile
func readPuppetfile(targetDir string) PuppetfileSettings {
	var puppetFile PuppetfileSettings
	puppetFile.forgeModules = map[string]ForgeModule{}
	puppetFile.gitModules = map[string]GitModule{}
	pf := targetDir + "Puppetfile"
	if _, err := os.Stat(pf); os.IsNotExist(err) {
	} else {
		//log.Print("Trying to parse: " + pf)
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
				if len(m[3]) > 1 {
					puppetFile.forgeModules[m[1]] = ForgeModule{version: m[3]}
					//fmt.Println("found forge mod attribute ---> ", m[3])
					//fmt.Println("found m[2] ---> '", m[2], "'")
				} else {
					//puppetFile.forgeModules[m[1]] = ForgeModule{}
					puppetFile.forgeModules[m[1]] = ForgeModule{}
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
	return puppetFile
}

func executeCommand(command string) string {
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
	Verbosef("Executing " + command + " took " + strconv.FormatFloat(time.Since(before).Seconds(), 'f', 5, 64) + "s")
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
		executeCommand("ssh-agent bash -c 'ssh-add " + sshPrivateKey + "; " + gitCmd + "'")
	} else {
		executeCommand(gitCmd)
	}
}

func resolvePuppetEnvironment() {
	uniqueGitModules = make(map[string]struct{})
	var gitKey string
	for source, sa := range config.Sources {
		wg.Add(1)
		go func(source string, sa Source) {
			defer wg.Done()
			Debugf("Puppet environment: " + source + " (remote=" + sa.Remote + ", basedir=" + sa.Basedir + ", private_key=" + sa.PrivateKey + ", prefix=" + strconv.FormatBool(sa.Prefix) + ")")
			workDir := config.CacheDir + source + ".git"
			// check if sa.Basedir exists
			checkDirAndCreate(sa.Basedir, "basedir")

			if !strings.Contains(source, "hiera") && !strings.Contains(source, "files") {
				gitKey = sa.PrivateKey
			}
			doMirrorOrUpdate(sa.Remote, workDir, sa.PrivateKey)

			// get all branches
			out := executeCommand("git --git-dir " + workDir + " for-each-ref --sort=-committerdate --format=%(refname:short)")
			//log.Print(branches)
			branches := strings.Split(out, "\n")
			for _, branch := range branches {
				wg.Add(1)
				go func(branch string) {
					defer wg.Done()
					if len(branch) != 0 {
						Debugf("Resolving branch:" + branch)
						// TODO if sa.Prefix != true
						targetDir := sa.Basedir + source + "_" + branch + "/"
						syncToModuleDir(workDir, targetDir, branch)
						if !strings.Contains(source, "hiera") && !strings.Contains(source, "files") {
							puppetfile := readPuppetfile(targetDir)
							createOrPurgeDir(puppetfile.moduleDir)
							//log.Println(targetDir, puppetfile)
							for _, git := range puppetfile.gitModules {
								//log.Println("inspecting", git.git)
								// collect unique git modules here over all branches
								if !containUniqeGitModule(git.git) {
									registerUniqeGitModule(git.git)
								}
							}
						}
					}
				}(branch)

			}
		}(source, sa)
	}

	wg.Wait()
	resolveGitRepositories(uniqueGitModules, gitKey)
	wgGit.Wait()
}

func registerUniqeGitModule(url string) {
	uniqueGitModulesMU.Lock()
	defer uniqueGitModulesMU.Unlock()
	uniqueGitModules[url] = empty
}

func containUniqeGitModule(url string) bool {
	uniqueGitModulesMU.Lock()
	defer uniqueGitModulesMU.Unlock()
	if _, ok := uniqueGitModules[url]; ok {
		return true
	} else {
		return false
	}
}

func resolveGitRepositories(uniqueGitModules map[string]struct{}, sshPrivateKey string) {
	//for n := range repos {
	for url, _ := range uniqueGitModules {
		wgGit.Add(1)
		go func(url string) {
			defer wgGit.Done()
			Debugf("git repo url " + url)

			// create save directory name from Git repo name
			repoDir := strings.Replace(strings.Replace(url, "/", "_", -1), ":", "-", -1)
			workDir := config.CacheDir + repoDir

			doMirrorOrUpdate(url, workDir, sshPrivateKey)
			//	doCloneOrPull(source, workDir, targetDir, sa.Remote, branch, sa.PrivateKey)

		}(url)
	}
	// wait for goroutines to finish
	//for i := 0; i < len(repos); i++ {
	//	<-sem
	//}
}

func createOrPurgeDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		Debugf("trying to create dir: " + dir)
		os.Mkdir(dir, 0777)
	} else {
		Debugf("Trying to remove: " + dir)
		errr := os.RemoveAll(dir)
		if errr != nil {
			log.Print("error: removing dir failed", errr)
		}
		Debugf("trying to create dir: " + dir)
		os.Mkdir(dir, 0777)
	}
}

func syncToModuleDir(srcDir string, targetDir string, tree string) {
	createOrPurgeDir(targetDir)
	cmd := "git --git-dir " + srcDir + " archive " + tree + " | tar -x -C " + targetDir
	before := time.Now()
	_, err := exec.Command("bash", "-c", cmd).Output()
	Verbosef("Executing " + cmd + " took " + strconv.FormatFloat(time.Since(before).Seconds(), 'f', 5, 64) + "s")
	if err != nil {
		log.Printf("Failed to execute command: %s", cmd)
	}
}

func resolveForgeModules(modules map[string]string) {
	for m := range modules {
		Debugf("Trying to get forge module " + m + " with " + modules[m] + config.CacheDir)
	}
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
	config = readConfigfile(*configFile)
	resolvePuppetEnvironment()

	// DEBUG
	//readPuppetfile("/tmp/core/core_fullmanaged/")
	//resolveGitRepositories(config)
	//resolveForgeModules(configSettings.forge)
}
