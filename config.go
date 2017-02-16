package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// readConfigfile creates the ConfigSettings struct from the g10k config file
func readConfigfile(configFile string) ConfigSettings {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		Fatalf("readConfigfile(): There was an error parsing the config file " + configFile + ": " + err.Error())
	}

	//fmt.Println("data:", string(data))
	data = bytes.Replace(data, []byte(":cachedir:"), []byte("cachedir:"), -1)
	//fmt.Println("data:", string(data))
	var config ConfigSettings
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		Fatalf("YAML unmarshal error: " + err.Error())
	}

	//fmt.Println("config:", config)
	//fmt.Println("config ----- forge:", config.Forge)
	//for k, v := range config.Sources {
	//	fmt.Print(k)
	//	fmt.Print(v.Remote)
	//}

	// check if cachedir exists
	config.CacheDir = checkDirAndCreate(config.CacheDir, "cachedir")
	config.ForgeCacheDir = checkDirAndCreate(config.CacheDir+"forge/", "cachedir/forge")
	config.ModulesCacheDir = checkDirAndCreate(config.CacheDir+"modules/", "cachedir/modules")
	config.EnvCacheDir = checkDirAndCreate(config.CacheDir+"environments/", "cachedir/environments")

	if len(config.Forge.Baseurl) == 0 {
		config.Forge.Baseurl = "https://forgeapi.puppetlabs.com"
	}

	//fmt.Println("Forge Baseurl: ", config.Forge.Baseurl)

	// set default timeout to 5 seconds if no timeout setting found
	if config.Timeout == 0 {
		config.Timeout = 5
	}

	return config
}

// preparePuppetfile remove whitespace and comment lines from the given Puppetfile and merges Puppetfile resources that are identified with having a , at the end
func preparePuppetfile(pf string) string {
	file, err := os.Open(pf)
	if err != nil {
		Fatalf("preparePuppetfile(): Error while opening Puppetfile " + pf + " Error: " + err.Error())
	}
	defer file.Close()

	reComment := regexp.MustCompile("^\\s*#")
	reEmpty := regexp.MustCompile("^$")

	pfString := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !reComment.MatchString(line) && !reEmpty.MatchString(line) {
			if strings.Contains(line, "#") {
				Debugf("found inline comment in " + pf + "line: " + line)
				line = strings.Split(line, "#")[0]
			}
			if regexp.MustCompile(",\\s*$").MatchString(line) {
				pfString += line
				Debugf("adding line:" + line)
			} else {
				pfString += line + "\n"
				Debugf("adding line:" + line)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		Fatalf("preparePuppetfile(): Error while scanning Puppetfile " + pf + " Error: " + err.Error())
	}

	return pfString
}

// readPuppetfile creates the ConfigSettings struct from the Puppetfile
func readPuppetfile(pf string, sshKey string, source string, forceForgeVersions bool) Puppetfile {
	var puppetFile Puppetfile
	puppetFile.privateKey = sshKey
	puppetFile.source = source
	puppetFile.forgeModules = map[string]ForgeModule{}
	puppetFile.gitModules = map[string]GitModule{}
	Debugf("Trying to parse: " + pf)

	n := preparePuppetfile(pf)

	reModuledir := regexp.MustCompile("^\\s*(?:moduledir)\\s+['\"]?([^'\"]+)['\"]?")
	reForgeCacheTtl := regexp.MustCompile("^\\s*(?:forge.cacheTtl)\\s+['\"]?([^'\"]+)['\"]?")
	reForgeBaseURL := regexp.MustCompile("^\\s*(?:forge.baseUrl)\\s+['\"]?([^'\"]+)['\"]?")
	reForgeModule := regexp.MustCompile("^\\s*(?:mod)\\s+['\"]?([^'\"]+/[^'\"]+)['\"](?:\\s*)[,]?(.*)")
	reForgeAttribute := regexp.MustCompile("\\s*['\"]?([^\\s'\"]+)\\s*['\"]?(?:=>)?\\s*['\"]?([^'\"]+)?")
	reGitModule := regexp.MustCompile("^\\s*(?:mod)\\s+['\"]?([^'\"/]+)['\"]\\s*,(.*)")
	reGitAttribute := regexp.MustCompile("\\s*:(git|commit|tag|branch|ref|link|ignore[-_]unreachable|fallback)\\s*=>\\s*['\"]?([^'\"]+)['\"]?")
	reUniqueGitAttribute := regexp.MustCompile("\\s*:(?:commit|tag|branch|ref|link)\\s*=>")
	//moduleName := ""
	//nextLineAttr := false

	for _, line := range strings.Split(n, "\n") {
		//fmt.Println("found line ---> ", line)
		if strings.Count(line, ":git") > 1 || strings.Count(line, ":tag") > 1 || strings.Count(line, ":branch") > 1 || strings.Count(line, ":ref") > 1 || strings.Count(line, ":link") > 1 {
			Fatalf("Error: trailing comma found in " + pf + " somewhere here: " + line)
		}
		if m := reModuledir.FindStringSubmatch(line); len(m) > 1 {
			puppetFile.moduleDir = m[1]
		} else if m := reForgeBaseURL.FindStringSubmatch(line); len(m) > 1 {
			puppetFile.forgeBaseURL = m[1]
			//fmt.Println("found forge base URL parameter ---> ", m[1])
		} else if m := reForgeCacheTtl.FindStringSubmatch(line); len(m) > 1 {
			ttl, err := time.ParseDuration(m[1])
			if err != nil {
				Fatalf("Error: Can not convert value " + m[1] + " of parameter " + m[0] + " to a golang Duration. Valid time units are 300ms, 1.5h or 2h45m. In " + pf + " line: " + line)
			}
			puppetFile.forgeCacheTtl = ttl
		} else if m := reForgeModule.FindStringSubmatch(line); len(m) > 1 {
			forgeModuleName := strings.TrimSpace(m[1])
			//fmt.Println("found forge mod name ---> ", forgeModuleName)
			comp := strings.Split(forgeModuleName, "/")
			if len(comp) != 2 {
				Fatalf("Error: Forge module name is invalid + should be like puppetlabs/apt + but is:" + m[2] + "in" + pf + "line: " + line)
			}
			if _, ok := puppetFile.forgeModules[forgeModuleName]; ok {
				Fatalf("Error: Duplicate forge module found in " + pf + " for module " + forgeModuleName + " line: " + line)
			}
			forgeModuleVersion := "present"
			forgeChecksum := ""
			// try to find a forge module attribute
			if len(m[2]) > 1 {
				forgeModuleAttributes := m[2]
				forgeModuleAttributesArray := strings.Split(forgeModuleAttributes, ",")
				//fmt.Println("found forge mod attribute array ---> ", forgeModuleAttributesArray)
				//fmt.Println("len(forgeModuleAttributesArray) --> ", len(forgeModuleAttributesArray))
				for i := 0; i <= strings.Count(forgeModuleAttributes, ","); i++ {
					a := reForgeAttribute.FindStringSubmatch(forgeModuleAttributesArray[i])
					//fmt.Println("a[1] ---> ", a[1])
					forgeAttribute := strings.Replace(strings.TrimSpace(a[1]), ":", "", 1)
					if forgeAttribute != "sha256sum" {
						forgeModuleVersion = forgeAttribute
						Debugf("setting forge module " + forgeModuleName + " to version " + forgeModuleVersion)
					}
					if len(a[2]) > 1 {
						//fmt.Println("a[2] ---> ", a[2])
						forgeAttributeName := strings.TrimSpace(a[1])
						forgeAttributeValue := strings.TrimSpace(a[2])
						Debugf("found forge attribute ---> " + forgeAttributeName + " with value ---> " + forgeAttributeValue)
						if forgeAttributeName == ":sha256sum" {
							forgeChecksum = forgeAttributeValue
						}
					}
				}
			}
			if forceForgeVersions && (forgeModuleVersion == "present" || forgeModuleVersion == "latest") {
				Fatalf("Error: Found " + forgeModuleVersion + " setting for forge module in " + pf + " for module " + forgeModuleName + " line: " + line + " and force_forge_versions is set to true! Please specify a version (e.g. '2.3.0')")
			}
			puppetFile.forgeModules[forgeModuleName] = ForgeModule{version: forgeModuleVersion, name: comp[1], author: comp[0], sha256sum: forgeChecksum}
		} else if m := reGitModule.FindStringSubmatch(line); len(m) > 1 {
			gitModuleName := m[1]
			//fmt.Println("found git mod name ---> ", gitModuleName)
			if strings.Contains(gitModuleName, "-") {
				Warnf("Warning: Found invalid character '-' in Puppet module name " + gitModuleName + " in " + pf + " line: " + line +
					"\n See module guidelines: https://docs.puppet.com/puppet/latest/reference/lang_reserved.html#modules")
			}
			if len(m[2]) > 1 {
				gitModuleAttributes := m[2]
				//fmt.Println("found git mod attribute ---> ", gitModuleAttributes)
				if strings.Count(gitModuleAttributes, ":git") < 1 {
					Fatalf("Error: Missing :git url in " + pf + " for module " + gitModuleName + " line: " + line)
				}
				if strings.Count(gitModuleAttributes, ",") > 3 {
					Fatalf("Error: Too many attributes in " + pf + " for module " + gitModuleName + " line: " + line)
				}
				if _, ok := puppetFile.gitModules[gitModuleName]; ok {
					Fatalf("Error: Duplicate module found in " + pf + " for module " + gitModuleName + " line: " + line)
				}
				gas := reUniqueGitAttribute.FindAllStringSubmatch(gitModuleAttributes, -1)
				cga := ""
				if len(gas) > 1 {
					for _, ga := range gas {
						cga += strings.TrimSpace(strings.Replace(ga[0], "=>", "", -1)) + ", "
					}
					Fatalf("Error: Found conflicting git attributes " + cga + "in " + pf + " for module " + gitModuleName + " line: " + line)
				}
				puppetFile.gitModules[gitModuleName] = GitModule{}
				gm := GitModule{}
				gitModuleAttributesArray := strings.Split(gitModuleAttributes, ",")
				//fmt.Println("found git mod attribute array ---> ", gitModuleAttributesArray)
				//fmt.Println("len(gitModuleAttributesArray) --> ", len(gitModuleAttributesArray))
				for i := 0; i <= strings.Count(gitModuleAttributes, ","); i++ {
					//fmt.Println("i -->", i)
					if i >= len(gitModuleAttributesArray) {
						Fatalf("Error: Trailing comma or invalid setting for module found in " + pf + " for module " + gitModuleName + " line: " + line)
					}
					a := reGitAttribute.FindStringSubmatch(gitModuleAttributesArray[i])
					//fmt.Println("a -->", a)
					if len(a) == 0 {
						Fatalf("Error: Trailing comma or invalid setting for module found in " + pf + " for module " + gitModuleName + " line: " + line)
					}
					gitModuleAttribute := a[1]
					if gitModuleAttribute == "git" {
						gm.git = a[2]
					} else if gitModuleAttribute == "branch" {
						gm.branch = a[2]
					} else if gitModuleAttribute == "tag" {
						gm.tag = a[2]
					} else if gitModuleAttribute == "commit" {
						gm.commit = a[2]
					} else if gitModuleAttribute == "ref" {
						gm.ref = a[2]
					} else if gitModuleAttribute == "link" {
						link, err := strconv.ParseBool(a[2])
						if err != nil {
							Fatalf("Error: Can not convert value " + a[2] + " of parameter " + gitModuleAttribute + " to boolean. In " + pf + " for module " + gitModuleName + " line: " + line)
						}
						gm.link = link
					} else if gitModuleAttribute == "ignore-unreachable" || gitModuleAttribute == "ignore_unreachable" {
						ignoreUnreachable, err := strconv.ParseBool(a[2])
						if err != nil {
							Fatalf("Error: Can not convert value " + a[2] + " of parameter " + gitModuleAttribute + " to boolean. In " + pf + " for module " + gitModuleName + " line: " + line)
						}
						gm.ignoreUnreachable = ignoreUnreachable
					} else if gitModuleAttribute == "fallback" {
						mapSize := strings.Count(a[2], "|") + 1
						gm.fallback = make([]string, mapSize)
						for i, fallbackBranch := range strings.Split(a[2], "|") {
							//fmt.Println("--------> ", i, strings.TrimSpace(fallbackBranch))
							gm.fallback[i] = strings.TrimSpace(fallbackBranch)
						}
					}

				}
				puppetFile.gitModules[gitModuleName] = gm
			}
		}

	}
	// check if we need to set defaults
	if len(moduleDirParam) != 0 {
		puppetFile.moduleDir = moduleDirParam
	} else {
		if len(puppetFile.moduleDir) == 0 {
			puppetFile.moduleDir = "modules"
		}
	}
	Debugf("Setting moduledir for Puppetfile " + pf + " to " + puppetFile.moduleDir)
	//fmt.Println(puppetFile)
	return puppetFile
}
