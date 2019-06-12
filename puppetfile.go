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
func sourceSanityCheck(source Source) {
	if len(source.PrivateKey) > 0 {
		if _, err := os.Stat(source.PrivateKey); err != nil {
			Fatalf("resolvePuppetEnvironment(): could not find SSH private key " + source.PrivateKey + " for source " + source.Name + " in config file " + configFile + " Error: " + err.Error())
		}
	}
	if len(source.Basedir) <= 0 {
		Fatalf("resolvePuppetEnvironment(): config setting basedir is not set for source " + source.Name + " in config file " + configFile)
	}
	if len(source.Remote) <= 0 {
		Fatalf("resolvePuppetEnvironment(): config setting remote is not set for source " + source.Name + " in config file " + configFile)
	}
}

func resolvePuppetEnvironment(envBranch string, tags bool, outputNameTag string) {
	wg := sizedwaitgroup.New(config.MaxExtractworker + 1)
	allPuppetfiles := make(map[string]Puppetfile)
	allEnvironments := make(map[string]bool)
	allBasedirs := make(map[string]bool)
	for sourceName, source := range config.Sources {
		source.Name = sourceName
		wg.Add()
		go func(source Source) {
			defer wg.Done()
			if force {
				createOrPurgeDir(source.Basedir, "resolvePuppetEnvironment()")
			}

			source.Basedir = checkDirAndCreate(source.Basedir, "basedir for source "+source.Name)
			Debugf("Puppet environment: " + source.Name + " (" + fmt.Sprintf("%+v", source) + ")")

			// check for a valid source that has all necessourcery attributes (basedir, remote, SSH key exist if given)
			sourceSanityCheck(source)

			workDir := config.EnvCacheDir + source.Name + ".git"
			// check if source.Basedir exists
			checkDirAndCreate(source.Basedir, "basedir")

			if success := doMirrorOrUpdate(source.Remote, workDir, source.PrivateKey, true, 1); success {

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
				for _, branch := range branches {
					branch = strings.TrimLeft(branch, "* ")
					reInvalidCharacters := regexp.MustCompile("\\W")
					if source.AutoCorrectEnvironmentNames == "error" && reInvalidCharacters.MatchString(branch) {
						Warnf("Ignoring branch " + branch + ", because it contains invalid characters")
						continue
					}
					// XXX: maybe make this user configurable (either with dedicated file or as YAML array in g10k config)
					if strings.Contains(branch, ";") || strings.Contains(branch, "&") || strings.Contains(branch, "|") || strings.HasPrefix(branch, "tmp/") && strings.HasSuffix(branch, "/head") || (len(envBranch) > 0 && branch != envBranch) {
						Debugf("Skipping branch " + branch)
						continue
					} else if len(envBranch) > 0 && branch == envBranch {
						foundBranch = true
					}

					wg.Add()

					go func(branch string, source Source) {
						defer wg.Done()
						if len(branch) != 0 {
							Debugf("Resolving branch: " + branch)

							renamedBranch := branch
							if (len(outputNameTag) > 0) && (len(envBranch) > 0) {
								renamedBranch = outputNameTag
								Debugf("Renaming branch " + branch + " to " + renamedBranch)
							}

							if source.AutoCorrectEnvironmentNames == "correct" || source.AutoCorrectEnvironmentNames == "correct_and_warn" {
								oldBranch := renamedBranch
								renamedBranch = reInvalidCharacters.ReplaceAllString(renamedBranch, "_")
								if source.AutoCorrectEnvironmentNames == "correct_and_warn" {
									if oldBranch != renamedBranch {
										Warnf("Renaming branch " + oldBranch + " to " + renamedBranch)
									}
								} else {
									Debugf("Renaming branch " + oldBranch + " to " + renamedBranch)
								}
							}

							targetDir := source.Basedir + source.Prefix + "_" + strings.Replace(renamedBranch, "/", "_", -1)
							if source.Prefix == "false" || source.Prefix == "" {
								targetDir = source.Basedir + strings.Replace(renamedBranch, "/", "_", -1)
							} else if source.Prefix == "true" {
								targetDir = source.Basedir + source.Name + "_" + strings.Replace(renamedBranch, "/", "_", -1)
							}
							targetDir = normalizeDir(targetDir)

							env := strings.Replace(strings.Replace(targetDir, source.Basedir, "", 1), "/", "", -1)
							syncToModuleDir(workDir, targetDir, branch, false, false, env)
							if !fileExists(targetDir + "Puppetfile") {
								Debugf("Skipping branch " + source.Name + "_" + branch + " because " + targetDir + "Puppetfile does not exist")
							} else {
								puppetfile := readPuppetfile(targetDir+"Puppetfile", source, false)
								puppetfile.workDir = normalizeDir(targetDir)
								puppetfile.controlRepoBranch = branch
								mutex.Lock()
								allPuppetfiles[env] = puppetfile
								allEnvironments[env] = true
								allBasedirs[source.Basedir] = true
								mutex.Unlock()

							}
						}
					}(branch, source)

				}

				if source.WarnMissingBranch && !foundBranch {
					Warnf("WARNING: Couldn't find specified branch '" + envBranch + "' anywhere in source '" + source.Name + "' (" + source.Remote + ")")
				}
			} else {
				Warnf("WARNING: Could not resolve git repository in source '" + source.Name + "' (" + source.Remote + ")")
				if source.ExitIfUnreachable == true {
					os.Exit(1)
				}
			}
		}(source)
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

	for source, sa := range config.Sources {
		// Clean up unknown environment directories
		if len(envBranch) == 0 {
			for basedir, _ := range allBasedirs {
				globPath := filepath.Join(basedir, sa.Prefix+"*")
				if sa.Prefix == "false" || sa.Prefix == "" {
					globPath = basedir + "*"
				} else if sa.Prefix == "true" {
					globPath = filepath.Join(basedir, source+"_*")
				} else {
					globPath = filepath.Join(basedir, sa.Prefix+"_"+"*")
				}

				Debugf("Glob'ing with path " + globPath)
				environments, _ := filepath.Glob(globPath)
				//		environments, _ = ioutil.ReadDir(basedir)
				//	}
				//fmt.Println(environments)
				//fmt.Println(allEnvironments)
				for _, env := range environments {
					envPath := strings.Split(env, "/")
					env = envPath[len(envPath)-1]
					Debugf("Checking if environment should exist: " + env)
					if allEnvironments[env] {
						Debugf("Leaving environment: " + env)
					} else {
						Debugf("Deleting environment: " + env)
						if !dryRun {
							purgeDir(basedir+env, "resolvePuppetEnvironment()")
						}
					}
				}
			}
		}
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
					success = syncToModuleDir(moduleCacheDir, targetDir, tree, true, gitModule.ignoreUnreachable, env)
				}

				if len(gitModule.fallback) > 0 {
					if !success {
						for i, fallbackBranch := range gitModule.fallback {
							if i == len(gitModule.fallback)-1 {
								// last try
								gitModule.ignoreUnreachable = true
							}
							Debugf("Trying to resolve " + moduleCacheDir + " with branch " + fallbackBranch)
							success = syncToModuleDir(moduleCacheDir, targetDir, fallbackBranch, true, gitModule.ignoreUnreachable, env)
							if success {
								break
							}
						}
					}
				} else {
					syncToModuleDir(moduleCacheDir, targetDir, tree, gitModule.ignoreUnreachable, gitModule.ignoreUnreachable, env)
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

	//fmt.Println(uniqueForgeModules)
	if len(exisitingModuleDirs) > 0 && len(moduleParam) == 0 {
		for d := range exisitingModuleDirs {
			if strings.HasSuffix(d, ".resource_types") && isDir(d) {
				continue
			}
			Debugf("Removing unmanaged file " + d)
			if err := os.RemoveAll(d); err != nil {
				Debugf("Error while trying to remove unmanaged file " + d)
			}
		}
	}
	if !debug && !verbose && !info && !quiet && terminal.IsTerminal(int(os.Stdout.Fd())) {
		uiprogress.Stop()
	}
}
