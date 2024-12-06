package main

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xorpaul/g10k/internal"
	"github.com/xorpaul/g10k/internal/fsutils"
	"github.com/xorpaul/g10k/internal/logging"
)

var (
	reModuledir = regexp.MustCompile(`^\s*(?:moduledir)\s+['\"]?([^'\"]+)['\"]?`)
)

// preparePuppetfile remove whitespace and comment lines from the given Puppetfile and merges Puppetfile resources that are identified with having a , at the end
func preparePuppetfile(pf string) string {
	file, err := os.Open(pf)
	if err != nil {
		logging.Fatalf("preparePuppetfile(): Error while opening Puppetfile " + pf + " Error: " + err.Error())
	}
	defer file.Close()

	reComma := regexp.MustCompile(`,\s*$`)
	reComment := regexp.MustCompile(`^\s*#`)
	reEmpty := regexp.MustCompile("^$")

	pfString := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !reComment.MatchString(line) && !reEmpty.MatchString(line) {
			if strings.Contains(line, "#") {
				logging.Debugf("found inline comment in " + pf + "line: " + line)
				line = strings.Split(line, "#")[0]
			}
			if reComma.MatchString(line) {
				pfString += line
				logging.Debugf("adding line:" + line)
			} else {
				pfString += line + "\n"
				logging.Debugf("adding line:" + line)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logging.Fatalf("preparePuppetfile(): Error while scanning Puppetfile " + pf + " Error: " + err.Error())
	}

	return pfString
}

// readPuppetfile creates the ConfigSettings struct from the Puppetfile
func readPuppetfile(pf string, sshKey string, source string, branch string, forceForgeVersions bool, replacedPuppetfileContent bool) Puppetfile {
	var puppetFile Puppetfile
	var n string
	puppetFile.privateKey = sshKey
	puppetFile.source = source
	puppetFile.forgeModules = map[string]ForgeModule{}
	puppetFile.gitModules = map[string]GitModule{}
	if replacedPuppetfileContent {
		logging.Debugf("Using replaced Puppetfile content, probably because a Git module was found in Forge notation")
		n = pf
	} else {
		logging.Debugf("Trying to parse: " + pf)
		n = preparePuppetfile(pf)
	}

	reEmptyLine := regexp.MustCompile(`^\s*$`)
	reForgeCacheTTL := regexp.MustCompile(`^\s*(?:forge.cache(?:TTL|Ttl))\s+['\"]?([^'\"]+)['\"]?`)
	reForgeBaseURL := regexp.MustCompile(`^\s*(?:forge.base(?:URL|Url))\s+['\"]?([^'\"]+)['\"]?`)
	reForgeModule := regexp.MustCompile(`^\s*(?:mod)\s+['\"]?([^'\"]+[-/][^'\"]+)['\"](?:\s*)[,]?(.*)`)
	reForgeAttribute := regexp.MustCompile(`\s*['\"]?([^\s'\"]+)\s*['\"]?(?:=>)?\s*['\"]?([^'\"]+)?`)
	reGitModule := regexp.MustCompile(`^\s*(?:mod)\s+['\"]?([^'\"/]+)['\"]\s*,(.*)`)
	reGitAttribute := regexp.MustCompile(`\s*:(git|commit|tag|branch|ref|link|ignore[-_]unreachable|fallback|install_path|default_branch|local|use_ssh_agent)\s*=>\s*['\"]?([^'\"]+)['\"]?`)
	reUniqueGitAttribute := regexp.MustCompile(`\s*:(?:commit|tag|branch|ref|link)\s*=>`)
	reDanglingAttribute := regexp.MustCompile(`^\s*:[^ ]+\s*=>`)
	moduleDir := "modules"
	// moduledir CLI parameter override
	if len(moduleDirParam) != 0 {
		moduleDir = moduleDirParam
	}
	var moduleDirs []string
	//nextLineAttr := false

	lines := strings.Split(n, "\n")
	for i, line := range lines {
		//fmt.Println("found line ---> ", line, "$")
		if m := reEmptyLine.FindStringSubmatch(line); len(m) > 0 {
			continue
		}
		if strings.Count(line, ":git") > 1 || strings.Count(line, ":tag") > 1 || strings.Count(line, ":branch") > 1 || strings.Count(line, ":ref") > 1 || strings.Count(line, ":link") > 1 {
			logging.Fatalf("Error: trailing comma found in " + pf + " somewhere here: " + line)
		}
		if m := reDanglingAttribute.FindStringSubmatch(line); len(m) >= 1 {
			previousLine := ""
			if i-1 >= 0 {
				previousLine = lines[i-1]
			}
			logging.Fatalf("Error: found dangling module attribute in " + pf + " somewhere here: " + previousLine + line + " Check for missing , at the end of the line.")
		}
		if m := reModuledir.FindStringSubmatch(line); len(m) > 1 && len(moduleDirParam) == 0 {
			moduleDir = fsutils.NormalizeDir(m[1])
			moduleDirs = append(moduleDirs, moduleDir)
		} else if m := reForgeBaseURL.FindStringSubmatch(line); len(m) > 1 {
			puppetFile.forgeBaseURL = m[1]
			//fmt.Println("found forge base URL parameter ---> ", m[1])
		} else if m := reForgeCacheTTL.FindStringSubmatch(line); len(m) > 1 {
			ttl, err := time.ParseDuration(m[1])
			if err != nil {
				logging.Fatalf("Error: Can not convert value " + m[1] + " of parameter " + m[0] + " to a golang Duration. Valid time units are 300ms, 1.5h or 2h45m. In " + pf + " line: " + line)
			}
			puppetFile.forgeCacheTTL = ttl
		} else if m := reForgeModule.FindStringSubmatch(line); len(m) > 1 {
			forgeModuleName := strings.TrimSpace(m[1])
			//fmt.Println("found forge mod name ------------------------------> ", forgeModuleName)
			comp := strings.Split(forgeModuleName, "/")
			forgeModuleNameSeparator := "/"
			if len(comp) != 2 {
				comp = strings.Split(forgeModuleName, "-")
				forgeModuleNameSeparator = "-"
				if len(comp) != 2 {
					logging.Fatalf("Error: Forge module name is invalid! Should be like puppetlabs/apt or puppetlabs-apt, but is: " + m[2] + " in " + pf + " line: " + line)
				}
			}
			forgeModuleName = comp[0] + "/" + comp[1]
			if _, ok := puppetFile.forgeModules[comp[1]]; ok {
				logging.Fatalf("Error: Duplicate forge module found in " + pf + " for module " + forgeModuleName + " line: " + line)
			}
			//logging.Debugf("Found Forge module name " + forgeModuleName + " with " + forgeModuleNameSeparator + " as a separator")
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
						logging.Debugf("setting forge module " + forgeModuleName + " to version " + forgeModuleVersion)
					}
					if len(a[2]) > 1 {
						//fmt.Println("a[2] ---> ", a[2])
						forgeAttributeName := strings.TrimSpace(a[1])
						forgeAttributeValue := strings.TrimSpace(a[2])
						logging.Debugf("found forge attribute ---> " + forgeAttributeName + " with value ---> " + forgeAttributeValue)
						if forgeAttributeName == ":sha256sum" {
							forgeChecksum = forgeAttributeValue
						} else if forgeAttribute == "git" {
							// try to detect Git modules in Forge <AUTHOR>/<MODULENAME> notation, fixes #104
							logging.Debugf("Found git module in Forge notation: " + forgeModuleName + " with git url: " + forgeAttributeValue)
							//fmt.Println("line:", line)
							removeForgeNotationAuthor := strings.Split(line, forgeModuleNameSeparator)
							if len(removeForgeNotationAuthor) < 2 {
								logging.Fatalf("Error: Found git module in Forge notation: " + forgeModuleName + " with git url: " + forgeAttributeValue + ", but something went wrong while trying to remove the author part to make g10k detect it as an Git module module:" + comp[1] + " line: " + line)
							} else {
								//fmt.Println("removeForgeNotationAuthor:", removeForgeNotationAuthor[0])
								replacedLine := strings.Replace(line, removeForgeNotationAuthor[0]+forgeModuleNameSeparator, "mod '", 1)
								//fmt.Println("replacedLine:", replacedLine)
								//fmt.Print("n:", n)
								newN := strings.Replace(n, line, replacedLine, 1)
								//fmt.Print("newN:", newN)
								return readPuppetfile(newN, sshKey, source, branch, forceForgeVersions, true)
							}
						}
					}
				}
			}
			if forceForgeVersions && (forgeModuleVersion == "present" || forgeModuleVersion == "latest") {
				logging.Fatalf("Error: Found " + forgeModuleVersion + " setting for forge module in " + pf + " for module " + forgeModuleName + " line: " + line + " and force_forge_versions is set to true! Please specify a version (e.g. '2.3.0')")
			}
			if _, ok := puppetFile.gitModules[comp[1]]; ok {
				logging.Fatalf("Error: Forge Puppet module with same name found in " + pf + " for module " + comp[1] + " line: " + line)
			}
			// the base url in the Puppetfile takes precedence over an base url specified in the g10k config yaml
			if len(puppetFile.forgeBaseURL) == 0 {
				puppetFile.forgeBaseURL = GlobalConfig.ForgeBaseURL
			}
			puppetFile.forgeModules[comp[1]] = ForgeModule{version: forgeModuleVersion, name: comp[1], author: comp[0], sha256sum: forgeChecksum, moduleDir: moduleDir, sourceBranch: source + "_" + branch}
		} else if m := reGitModule.FindStringSubmatch(line); len(m) > 1 {
			gitModuleName := m[1]
			//fmt.Println("found git mod name ---> ", gitModuleName)
			if strings.Contains(gitModuleName, "-") {
				logging.Warnf("Warning: Found invalid character '-' in Puppet module name " + gitModuleName + " in " + pf + " line: " + line +
					"\n See module guidelines: https://docs.puppet.com/puppet/latest/reference/lang_reserved.html#modules")
			}
			if len(m[2]) > 1 {
				gitModuleAttributes := m[2]
				//fmt.Println("found git mod attribute ---> ", gitModuleAttributes)
				if strings.Count(gitModuleAttributes, ":git") < 1 && strings.Count(gitModuleAttributes, ":local") < 1 {
					logging.Fatalf("Error: Missing :git url in " + pf + " for module " + gitModuleName + " line: " + line)
				}
				if strings.Count(gitModuleAttributes, ",") > 3 {
					logging.Fatalf("Error: Too many attributes in " + pf + " for module " + gitModuleName + " line: " + line)
				}
				if _, ok := puppetFile.gitModules[gitModuleName]; ok {
					logging.Fatalf("Error: Duplicate module found in " + pf + " for module " + gitModuleName + " line: " + line)
				}
				gas := reUniqueGitAttribute.FindAllStringSubmatch(gitModuleAttributes, -1)
				cga := ""
				if len(gas) > 1 {
					for _, ga := range gas {
						cga += strings.TrimSpace(strings.Replace(ga[0], "=>", "", -1)) + ", "
					}
					logging.Fatalf("Error: Found conflicting git attributes " + cga + "in " + pf + " for module " + gitModuleName + " line: " + line)
				}
				puppetFile.gitModules[gitModuleName] = GitModule{}
				gm := GitModule{moduleDir: moduleDir}
				gitModuleAttributesArray := strings.Split(gitModuleAttributes, ",")
				//fmt.Println("found git mod attribute array ---> ", gitModuleAttributesArray)
				//fmt.Println("len(gitModuleAttributesArray) --> ", len(gitModuleAttributesArray))
				for i := 0; i <= strings.Count(gitModuleAttributes, ","); i++ {
					//fmt.Println("i -->", i)
					if i >= len(gitModuleAttributesArray) {
						logging.Fatalf("Error: Trailing comma or invalid setting for module found in " + pf + " for module " + gitModuleName + " line: " + line)
					}
					a := reGitAttribute.FindStringSubmatch(gitModuleAttributesArray[i])
					//fmt.Println("a -->", a)
					if len(a) == 0 {
						logging.Fatalf("Error: Trailing comma or invalid setting for module found in " + pf + " for module " + gitModuleName + " line: " + line)
					}
					gitModuleAttribute := a[1]
					if gitModuleAttribute == "git" {
						if strings.Contains(a[2], "ProxyCommand") {
							logging.Fatalf("Error: Found ProxyCommand option in git url in " + pf + " for module " + gitModuleName + " line: " + line)
						}
						gm.git = a[2]
					} else if gitModuleAttribute == "branch" {
						if a[2] == ":control_branch" || a[2] == "control_branch" {
							gm.link = true
						} else {
							gm.branch = a[2]
						}
					} else if gitModuleAttribute == "tag" {
						gm.tag = a[2]
					} else if gitModuleAttribute == "commit" {
						gm.commit = a[2]
					} else if gitModuleAttribute == "ref" {
						gm.ref = a[2]
					} else if gitModuleAttribute == "install_path" {
						gm.installPath = a[2]
					} else if gitModuleAttribute == "link" {
						link, err := strconv.ParseBool(a[2])
						if err != nil {
							logging.Fatalf("Error: Can not convert value " + a[2] + " of parameter " + gitModuleAttribute + " to boolean. In " + pf + " for module " + gitModuleName + " line: " + line)
						}
						gm.link = link
					} else if gitModuleAttribute == "ignore-unreachable" || gitModuleAttribute == "ignore_unreachable" {
						ignoreUnreachable, err := strconv.ParseBool(a[2])
						if err != nil {
							logging.Fatalf("Error: Can not convert value " + a[2] + " of parameter " + gitModuleAttribute + " to boolean. In " + pf + " for module " + gitModuleName + " line: " + line)
						}
						gm.ignoreUnreachable = ignoreUnreachable
					} else if gitModuleAttribute == "fallback" || gitModuleAttribute == "default_branch" {
						mapSize := strings.Count(a[2], "|") + 1
						gm.fallback = make([]string, mapSize)
						for i, fallbackBranch := range strings.Split(a[2], "|") {
							//fmt.Println("--------> ", i, strings.TrimSpace(fallbackBranch))
							gm.fallback[i] = strings.TrimSpace(fallbackBranch)
						}
					} else if gitModuleAttribute == "local" {
						local, err := strconv.ParseBool(a[2])
						if err != nil {
							logging.Fatalf("Error: Can not convert value " + a[2] + " of parameter " + gitModuleAttribute + " to boolean. In " + pf + " for module " + gitModuleName + " line: " + line)
						}
						if local {
							gm.local = true
						}
					} else if gitModuleAttribute == "use_ssh_agent" {
						useSSHAgent, err := strconv.ParseBool(a[2])
						if err != nil {
							logging.Fatalf("Error: Can not convert value " + a[2] + " of parameter " + gitModuleAttribute + " to boolean. In " + pf + " for module " + gitModuleName + " line: " + line)
						}
						gm.useSSHAgent = useSSHAgent
					}

				}
				if _, ok := puppetFile.forgeModules[gitModuleName]; ok {
					logging.Fatalf("Error: Git Puppet module with same name found in " + pf + " for module " + gitModuleName + " line: " + line)
				}
				if GlobalConfig.IgnoreUnreachableModules {
					logging.Debugf("Setting :ignore_unreachable for Git module " + gitModuleName)
					gm.ignoreUnreachable = true
				}
				puppetFile.gitModules[gitModuleName] = gm
			}
		} else {
			// for now only in dry run mode
			if internal.DryRun {
				logging.Fatalf("Error: Could not interpret line: " + line + " In " + pf)
			}

		}

	}

	if len(moduleDirs) < 1 {
		// adding at least the default module directory
		moduleDirs = append(moduleDirs, moduleDir)
	}

	if logging.Validate {
		logging.Validatef()
	}

	puppetFile.moduleDirs = moduleDirs
	puppetFile.sourceBranch = branch
	// fmt.Printf("%+v\n", puppetFile)
	return puppetFile
}
