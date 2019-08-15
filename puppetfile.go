package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/remeh/sizedwaitgroup"
	"github.com/xorpaul/uiprogress"
	"golang.org/x/crypto/ssh/terminal"
)

// sourceSanityCheck is a validation function that checks if the given source has all necessary attributes (basedir, remote, SSH key exists if given)
func sourceSanityCheck(source string, sa Source) {
	if len(sa.PrivateKey) > 0 {
		if _, err := os.Stat(sa.PrivateKey); err != nil {
			Fatalf("resolvePuppetEnvironment(): could not find SSH private key " + sa.PrivateKey + " for source " + source + " in config file " + configFile + " Error: " + err.Error())
		}
	}
	if len(sa.Basedir) <= 0 {
		Fatalf("resolvePuppetEnvironment(): config setting basedir is not set for source " + source + " in config file " + configFile)
	}
	if len(sa.Remote) <= 0 {
		Fatalf("resolvePuppetEnvironment(): config setting remote is not set for source " + source + " in config file " + configFile)
	}
}

func resolvePuppetEnvironment(envBranch string, tags bool, outputNameTag string) {
	wg := sizedwaitgroup.New(config.MaxExtractworker + 1)
	allPuppetfiles := make(map[string]Puppetfile)
	allEnvironments := make(map[string]bool)
	allBasedirs := make(map[string]bool)
	for source, sa := range config.Sources {
		wg.Add()
		go func(source string, sa Source) {
			defer wg.Done()
			if force {
				createOrPurgeDir(sa.Basedir, "resolvePuppetEnvironment()")
			}

			sa.Basedir = checkDirAndCreate(sa.Basedir, "basedir for source "+source)
			Debugf("Puppet environment: " + source + " (" + fmt.Sprintf("%+v", sa) + ")")

			// check for a valid source that has all necessary attributes (basedir, remote, SSH key exist if given)
			sourceSanityCheck(source, sa)

			workDir := config.EnvCacheDir + source + ".git"
			// check if sa.Basedir exists
			checkDirAndCreate(sa.Basedir, "basedir")

			if success := doMirrorOrUpdate(sa.Remote, workDir, sa.PrivateKey, true, 1); success {

				// get all branches
				er := executeCommand("git --git-dir "+workDir+" branch", config.Timeout, false)
				outputBranches := er.output
				outputTags := ""

				if tags == true {
					er := executeCommand("git --git-dir "+workDir+" tag", config.Timeout, false)
					outputTags = er.output
				}

				branches := strings.Split(strings.TrimSpace(outputBranches+outputTags), "\n")

				foundBranch := false
				prefix := resolveSourcePrefix(source, sa)
				for _, branch := range branches {
					branch = strings.TrimLeft(branch, "* ")
					reInvalidCharacters := regexp.MustCompile("\\W")
					if sa.AutoCorrectEnvironmentNames == "error" && reInvalidCharacters.MatchString(branch) {
						Warnf("Ignoring branch " + branch + ", because it contains invalid characters")
						continue
					}
					// XXX: maybe make this user configurable (either with dedicated file or as YAML array in g10k config)
					if strings.Contains(branch, ";") || strings.Contains(branch, "&") || strings.Contains(branch, "|") || strings.HasPrefix(branch, "tmp/") && strings.HasSuffix(branch, "/head") {
						Debugf("Skipping branch " + branch + ", because of invalid character(s) inside the branch name")
						continue
					}
					if len(envBranch) > 0 {
						if branch == envBranch {
							foundBranch = true
						} else {
							Debugf("Skipping branch " + branch)
							continue
						}
					}

					wg.Add()

					go func(branch string, sa Source, prefix string) {
						defer wg.Done()
						if len(branch) != 0 {
							Debugf("Resolving branch: " + branch)

							renamedBranch := branch
							if (len(outputNameTag) > 0) && (len(envBranch) > 0) {
								renamedBranch = outputNameTag
								Debugf("Renaming branch " + branch + " to " + renamedBranch)
							}

							if sa.AutoCorrectEnvironmentNames == "correct" || sa.AutoCorrectEnvironmentNames == "correct_and_warn" {
								oldBranch := renamedBranch
								renamedBranch = reInvalidCharacters.ReplaceAllString(renamedBranch, "_")
								if sa.AutoCorrectEnvironmentNames == "correct_and_warn" {
									if oldBranch != renamedBranch {
										Warnf("Renaming branch " + oldBranch + " to " + renamedBranch)
									}
								} else {
									Debugf("Renaming branch " + oldBranch + " to " + renamedBranch)
								}
							}

							targetDir := sa.Basedir + prefix + strings.Replace(renamedBranch, "/", "_", -1)
							targetDir = normalizeDir(targetDir)

							env := strings.Replace(strings.Replace(targetDir, sa.Basedir, "", 1), "/", "", -1)
							syncToModuleDir(workDir, targetDir, branch, false, false, env, true)
							if !fileExists(targetDir + "Puppetfile") {
								Debugf("Skipping branch " + source + "_" + branch + " because " + targetDir + "Puppetfile does not exist")
							} else {
								puppetfile := readPuppetfile(targetDir+"Puppetfile", sa.PrivateKey, source, sa.ForceForgeVersions, false)
								puppetfile.workDir = normalizeDir(targetDir)
								puppetfile.controlRepoBranch = branch
								mutex.Lock()
								for _, moduleDir := range puppetfile.moduleDirs {
									desiredContent = append(desiredContent, filepath.Join(puppetfile.workDir, moduleDir))
								}
								allPuppetfiles[env] = puppetfile
								allEnvironments[env] = true
								allBasedirs[sa.Basedir] = true
								mutex.Unlock()

							}
						}
					}(branch, sa, prefix)
				}

				if sa.WarnMissingBranch && !foundBranch {
					Warnf("WARNING: Couldn't find specified branch '" + envBranch + "' anywhere in source '" + source + "' (" + sa.Remote + ")")
				}
			} else {
				Warnf("WARNING: Could not resolve git repository in source '" + source + "' (" + sa.Remote + ")")
				if sa.ExitIfUnreachable == true {
					os.Exit(1)
				}
			}
		}(source, sa)
	}

	wg.Wait()
	//fmt.Println("allPuppetfiles: ", allPuppetfiles, len(allPuppetfiles))
	//fmt.Println("allPuppetfiles[0]: ", allPuppetfiles["postinstall"])
	resolvePuppetfile(allPuppetfiles)
	//fmt.Println(desiredContent)
	purgeUnmanagedContent(envBranch, allBasedirs, allEnvironments)

}

func purgeUnmanagedContent(envBranch string, allBasedirs map[string]bool, allEnvironments map[string]bool) {
	if !stringSliceContains(config.PurgeLevels, "deployment") {
		if !stringSliceContains(config.PurgeLevels, "environment") {
			// nothing allowed to purge
			return
		}
	}

	for source, sa := range config.Sources {
		prefix := resolveSourcePrefix(source, sa)
		// Clean up unknown environment directories
		if len(envBranch) == 0 {
			for basedir, _ := range allBasedirs {
				globPath := filepath.Join(basedir, prefix+"*")
				Debugf("Glob'ing with path " + globPath)
				environments, _ := filepath.Glob(globPath)

				whitelistEnvironments := []string{}
				if len(config.DeploymentPurgeWhitelist) > 0 {
					for _, wlpattern := range config.DeploymentPurgeWhitelist {
						whitelistGlobPath := filepath.Join(basedir, wlpattern)
						Debugf("deployment_purge_whitelist Glob'ing with path " + whitelistGlobPath)
						we, _ := filepath.Glob(whitelistGlobPath)
						whitelistEnvironments = append(whitelistEnvironments, we...)
					}
				}

				for _, env := range environments {
					envPath := strings.Split(env, "/")
					envName := envPath[len(envPath)-1]
					if stringSliceContains(config.PurgeLevels, "environment") {
						if allEnvironments[envName] {
							checkForStaleContent(env)
						}
					}
					if stringSliceContains(config.PurgeLevels, "deployment") {
						Debugf("Checking if environment should exist: " + envName)
						if allEnvironments[envName] {
							Debugf("Not purging environment " + envName)
						} else if stringSliceContains(whitelistEnvironments, filepath.Join(basedir, envName)) {
							Debugf("Not purging environment " + envName + " due to deployment_purge_whitelist match")
						} else {
							Infof("Removing unmanaged environment " + envName)
							if !dryRun {
								purgeDir(filepath.Join(basedir, envName), "purgeStaleContent()")
							}
						}
					}
				}
			}
		} else {
			if stringSliceContains(config.PurgeLevels, "environment") {
				// check for purgeable content inside -branch folder
				checkForStaleContent(filepath.Join(sa.Basedir, prefix+envBranch))
			}
		}
	}
}

func checkForStaleContent(workDir string) {
	// add purge whitelist
	if len(config.PurgeWhitelist) > 0 {
		Debugf("additional purge whitelist items: " + strings.Join(config.PurgeWhitelist, " "))
		for _, wlItem := range config.PurgeWhitelist {
			desiredContent = append(desiredContent, filepath.Join(workDir, wlItem))
		}
	}

	checkForStaleContent := func(path string, info os.FileInfo, err error) error {
		stale := true
		for _, desiredFile := range desiredContent {
			if strings.HasPrefix(path, desiredFile) || path == workDir {
				stale = false
			}
		}

		if stale {
			Infof("Removing unmanaged path " + path)
			purgeDir(path, "checkForStaleContent()")
		}
		return nil
	}

	c := make(chan error)
	Debugf("filepath.Walk'ing directory " + workDir)
	go func() { c <- filepath.Walk(workDir, checkForStaleContent) }()
	<-c // Walk done
}

// resolvePuppetEnvironment implements the prefix read out from each source given in the config file, like r10k https://github.com/puppetlabs/r10k/blob/master/doc/dynamic-environments/configuration.mkd#prefix
func resolveSourcePrefix(source string, sa Source) string {
	if sa.Prefix == "false" || sa.Prefix == "" {
		return ""
	} else if sa.Prefix == "true" {
		return source + "_"
	} else {
		return sa.Prefix + "_"
	}
}

func resolvePuppetfile(allPuppetfiles map[string]Puppetfile) {
	wg := sizedwaitgroup.New(config.MaxExtractworker)
	exisitingModuleDirs := make(map[string]struct{})
	uniqueGitModules := make(map[string]GitModule)
	// if we made it this far initialize the global maps
	latestForgeModules.m = make(map[string]string)
	for env, pf := range allPuppetfiles {
		Debugf("Resolving " + env)
		//fmt.Println(pf)
		for gitName, gitModule := range pf.gitModules {
			if len(moduleParam) > 0 {
				if gitName != moduleParam {
					Debugf("Skipping git module " + gitName + ", because parameter -module is set to " + moduleParam)
					delete(pf.gitModules, gitName)
					continue
				}
			}
			if gitModule.local {
				continue
			}

			gitModule.privateKey = pf.privateKey
			if _, ok := uniqueGitModules[gitModule.git]; !ok {
				uniqueGitModules[gitModule.git] = gitModule
			}
		}
		for forgeModuleName, fm := range pf.forgeModules {
			if len(moduleParam) > 0 {
				if forgeModuleName != moduleParam {
					Debugf("Skipping forge module " + forgeModuleName + ", because parameter -module is set to " + moduleParam)
					delete(pf.forgeModules, forgeModuleName)
					continue
				}
			}
			//fmt.Println("Found Forge module ", fm.author, "/", forgeModuleName, " with version", fm.version)
			fm.baseURL = pf.forgeBaseURL
			fm.cacheTTL = pf.forgeCacheTTL
			forgeModuleName = strings.Replace(forgeModuleName, "/", "-", -1)
			uniqueForgeModuleName := fm.author + "/" + forgeModuleName + "-" + fm.version
			if _, ok := uniqueForgeModules[uniqueForgeModuleName]; !ok {
				uniqueForgeModules[uniqueForgeModuleName] = fm
			} else {
				// Use the shortest Forge cache TTL for this module
				if uniqueForgeModules[uniqueForgeModuleName].cacheTTL > pf.forgeCacheTTL {
					delete(uniqueForgeModules, uniqueForgeModuleName)
					uniqueForgeModules[uniqueForgeModuleName] = fm
				}
			}
		}
	}
	if !debug && !verbose && !info && !quiet && terminal.IsTerminal(int(os.Stdout.Fd())) {
		uiprogress.Start()
	}
	var wgResolve sync.WaitGroup
	wgResolve.Add(2)
	go func() {
		defer wgResolve.Done()
		resolveGitRepositories(uniqueGitModules)
	}()
	go func() {
		defer wgResolve.Done()
		resolveForgeModules(uniqueForgeModules)
	}()
	wgResolve.Wait()
	//log.Println(config.Sources["cmdlineparam"])
	for env, pf := range allPuppetfiles {
		Debugf("Syncing " + env + " with workDir " + pf.workDir)
		basedir := checkDirAndCreate(pf.workDir, "basedir 2 for source "+pf.source)
		var envBranch string
		if !pfMode {
			// we want only the branch name of the control repo and not the resulting
			// Puppet environment folder name, which could contain a prefix
			envBranch = pf.controlRepoBranch
		}

		for _, moduleDir := range pf.moduleDirs {
			exisitingModuleDirsFI, _ := ioutil.ReadDir(pf.workDir + moduleDir)
			moduleDir = normalizeDir(pf.workDir + moduleDir)
			//fmt.Println("checking dir: ", moduleDir)
			mutex.Lock()
			for _, exisitingModuleDir := range exisitingModuleDirsFI {
				//fmt.Println("adding dir: ", moduleDir+exisitingModuleDir.Name())
				exisitingModuleDirs[moduleDir+exisitingModuleDir.Name()] = empty
			}
			mutex.Unlock()
		}

		for gitName, gitModule := range pf.gitModules {
			moduleDir := pf.workDir + gitModule.moduleDir
			moduleDir = normalizeDir(moduleDir)
			if gitModule.local {
				Debugf("Not deleting " + moduleDir + gitName + " as it is declared as a local module")
				// remove this module from the exisitingModuleDirs map
				moduleDirectory := moduleDir + gitName
				if len(gitModule.installPath) > 0 {
					moduleDirectory = normalizeDir(basedir) + normalizeDir(gitModule.installPath) + gitName
				}
				moduleDirectory = normalizeDir(moduleDirectory)
				mutex.Lock()
				if _, ok := exisitingModuleDirs[moduleDirectory]; ok {
					delete(exisitingModuleDirs, moduleDirectory)
				}
				for existingDir := range exisitingModuleDirs {
					rel, _ := filepath.Rel(existingDir, moduleDirectory)
					if len(rel) > 0 && !strings.Contains(rel, "..") {
						Debugf("not removing moduleDirectory " + moduleDirectory + " because it's a subdirectory to existingDir " + existingDir)
						delete(exisitingModuleDirs, existingDir)
					}
				}
				mutex.Unlock()
				continue
			}
			wg.Add()
			go func(gitName string, gitModule GitModule, env string) {
				defer wg.Done()
				targetDir := normalizeDir(moduleDir + gitName)
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
				} else if gitModule.link {
					if pfMode {
						if len(os.Getenv("g10k_branch")) > 0 {
							tree = os.Getenv("g10k_branch")
						} else if len(branchParam) > 0 {
							tree = branchParam
						} else {
							Fatalf("resolvePuppetfile(): found module " + gitName + " with module link mode enabled and g10k in Puppetfile mode which is not supported, as g10k can not detect the environment branch of the Puppetfile. You can explicitly set the module link branch you want to use in Puppetfile mode by setting the environment variable 'g10k_branch' or using the -branch parameter")
						}
					} else {
						tree = envBranch
					}
				}

				if len(gitModule.installPath) > 0 {
					targetDir = basedir + normalizeDir(gitModule.installPath) + gitName
				}
				targetDir = normalizeDir(targetDir)
				success := false
				moduleCacheDir := config.ModulesCacheDir + strings.Replace(strings.Replace(gitModule.git, "/", "_", -1), ":", "-", -1)

				if gitModule.link {
					Debugf("Trying to resolve " + moduleCacheDir + " with branch " + tree)
					success = syncToModuleDir(moduleCacheDir, targetDir, tree, true, gitModule.ignoreUnreachable, env, false)
				}

				if len(gitModule.fallback) > 0 {
					if !success {
						for i, fallbackBranch := range gitModule.fallback {
							if i == len(gitModule.fallback)-1 {
								// last try
								gitModule.ignoreUnreachable = true
							}
							Debugf("Trying to resolve " + moduleCacheDir + " with branch " + fallbackBranch)
							success = syncToModuleDir(moduleCacheDir, targetDir, fallbackBranch, true, gitModule.ignoreUnreachable, env, false)
							if success {
								break
							}
						}
					}
				} else {
					syncToModuleDir(moduleCacheDir, targetDir, tree, gitModule.ignoreUnreachable, gitModule.ignoreUnreachable, env, false)
				}

				// remove this module from the exisitingModuleDirs map
				moduleDirectory := moduleDir + gitName
				if len(gitModule.installPath) > 0 {
					moduleDirectory = normalizeDir(basedir) + normalizeDir(gitModule.installPath) + gitName
				}
				moduleDirectory = normalizeDir(moduleDirectory)
				mutex.Lock()
				if _, ok := exisitingModuleDirs[moduleDirectory]; ok {
					delete(exisitingModuleDirs, moduleDirectory)
				}
				for existingDir := range exisitingModuleDirs {
					rel, _ := filepath.Rel(existingDir, moduleDirectory)
					if len(rel) > 0 && !strings.Contains(rel, "..") {
						Debugf("not removing moduleDirectory " + moduleDirectory + " because it's a subdirectory to existingDir " + existingDir)
						delete(exisitingModuleDirs, existingDir)
					}
				}
				mutex.Unlock()
			}(gitName, gitModule, env)
		}
		for forgeModuleName, fm := range pf.forgeModules {
			wg.Add()
			moduleDir := pf.workDir + fm.moduleDir
			moduleDir = normalizeDir(moduleDir)
			go func(forgeModuleName string, fm ForgeModule, moduleDir string, env string) {
				defer wg.Done()
				syncForgeToModuleDir(forgeModuleName, fm, moduleDir, env)
				// remove this module from the exisitingModuleDirs map
				mutex.Lock()
				if _, ok := exisitingModuleDirs[moduleDir+fm.name]; ok {
					delete(exisitingModuleDirs, moduleDir+fm.name)
				}
				mutex.Unlock()
			}(forgeModuleName, fm, moduleDir, env)
		}

	}
	wg.Wait()

	if stringSliceContains(config.PurgeLevels, "puppetfile") {
		//fmt.Println(uniqueForgeModules)
		if len(exisitingModuleDirs) > 0 && len(moduleParam) == 0 {
			for d := range exisitingModuleDirs {
				if strings.HasSuffix(d, ".resource_types") && isDir(d) {
					continue
				}
				Infof("Removing unmanaged path " + d)
				if !dryRun {
					purgeDir(d, "purge_level puppetfile")
				}
			}
		}
	}
	if !debug && !verbose && !info && !quiet && terminal.IsTerminal(int(os.Stdout.Fd())) {
		uiprogress.Stop()
	}
}
