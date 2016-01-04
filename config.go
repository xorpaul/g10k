package main

import (
	"bufio"
	"bytes"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

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
func readPuppetfile(pf string, sshKey string) Puppetfile {
	var puppetFile Puppetfile
	puppetFile.privateKey = sshKey
	puppetFile.forgeModules = map[string]ForgeModule{}
	puppetFile.gitModules = map[string]GitModule{}
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
		//fmt.Println("found line ---> ", line)
		if strings.Count(line, ":git") > 1 || strings.Count(line, ":tag") > 1 || strings.Count(line, ":branch") > 1 || strings.Count(line, ":ref") > 1 {
			log.Fatal("Error: trailing comma found in", pf, "somewhere here: ", line)
			os.Exit(1)
		}
		if m := reModuledir.FindStringSubmatch(line); len(m) > 1 {
			puppetFile.moduleDir = m[1]
		} else if m := reForgeModule.FindStringSubmatch(line); len(m) > 1 {
			//fmt.Println("found forge mod name ---> ", m[1])
			comp := strings.Split(m[1], "/")
			if len(comp) != 2 {
				log.Print("Forge module name is invalid, should be like puppetlabs/apt, but is:", m[3], "in", pf, "line: ", line)
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
				fmt.Println("Warning: Found invalid character '-' in Puppet module name", m[1], "in", pf, "line:", line)
			}
			if len(m[2]) > 1 {
				gitModuleAttributes := m[2]
				if strings.Count(gitModuleAttributes, ":git") < 1 {
					log.Fatal("Error: Missing :git url in ", pf, " for module ", m[1], " line: ", line)
					os.Exit(1)
				}
				if strings.Count(gitModuleAttributes, ",") > 2 {
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
	// check if we need to set defaults
	if len(puppetFile.moduleDir) == 0 {
		puppetFile.moduleDir = "modules"
	}
	//fmt.Println(puppetFile)
	return puppetFile
}
