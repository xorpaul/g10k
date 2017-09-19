package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/henvic/uiprogress"
)

// sourceSanityCheck is a validation function that checks if the given source has all neccessary attributes (basedir, remote, SSH key exists if given)
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
			Debugf("Puppet environment: " + source + " (" + fmt.Sprintf("%+v", sa) + ")")

			// check for a valid source that has all neccessary attributes (basedir, remote, SSH key exists if given)
			sourceSanityCheck(source, sa)

			workDir := config.EnvCacheDir + source + ".git"
			// check if sa.Basedir exists
			checkDirAndCreate(sa.Basedir, "basedir")

			//if !strings.Contains(source, "hiera") && !strings.Contains(source, "files") {
			//	gitKey = sa.PrivateKey
			//}
			if success := doMirrorOrUpdate(sa.Remote, workDir, sa.PrivateKey, true); success {

				// get all branches
				er := executeCommand("git --git-dir "+workDir+" branch", config.Timeout, false)
				branches := strings.Split(strings.TrimSpace(er.output), "\n")

				foundBranch := false
				for _, branch := range branches {
					branch = strings.TrimLeft(branch, "* ")
					// XXX: maybe make this user configurable (either with dedicated file or as YAML array in g10k config)
					if strings.Contains(branch, ";") || strings.Contains(branch, "&") || strings.Contains(branch, "|") || strings.HasPrefix(branch, "tmp/") && strings.HasSuffix(branch, "/head") || (len(envBranch) > 0 && branch != envBranch) {
						Debugf("Skipping branch " + branch)
						continue
					} else if len(envBranch) > 0 && branch == envBranch {
						foundBranch = true
					}

					wg.Add(1)

					go func(branch string) {
						defer wg.Done()
						if len(branch) != 0 {
							Debugf("Resolving branch: " + branch)

							targetDir := sa.Basedir + sa.Prefix + "_" + strings.Replace(branch, "/", "_", -1) + "/"
							if sa.Prefix == "false" || sa.Prefix == "" {
								targetDir = sa.Basedir + strings.Replace(branch, "/", "_", -1) + "/"
							} else if sa.Prefix == "true" {
								targetDir = sa.Basedir + source + "_" + strings.Replace(branch, "/", "_", -1) + "/"
							}

							syncToModuleDir(workDir, targetDir, branch, false, false)
							if !fileExists(targetDir + "Puppetfile") {
								Debugf("Skipping branch " + source + "_" + branch + " because " + targetDir + "Puppetfile does not exitst")
							} else {
								puppetfile := readPuppetfile(targetDir+"Puppetfile", sa.PrivateKey, source, sa.ForceForgeVersions)
								puppetfile.workDir = targetDir
								mutex.Lock()
								allPuppetfiles[source+"_"+branch] = puppetfile
								mutex.Unlock()

							}
						}
					}(branch)

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
	exisitingModuleDirs := make(map[string]struct{})
	uniqueGitModules := make(map[string]GitModule)
	uniqueForgeModules := make(map[string]ForgeModule)
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
			fm.baseUrl = pf.forgeBaseURL
			fm.cacheTtl = pf.forgeCacheTtl
			forgeModuleName = strings.Replace(forgeModuleName, "/", "-", -1)
			if _, ok := uniqueForgeModules[fm.author+"/"+forgeModuleName+"-"+fm.version]; !ok {
				uniqueForgeModules[fm.author+"/"+forgeModuleName+"-"+fm.version] = fm
			}
		}
	}
	if !debug && !verbose && !info && !quiet {
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
		moduleDir := pf.workDir + pf.moduleDir
		var envBranch string
		if pfMode {
			moduleDir = basedir + "/" + pf.moduleDir
		} else {
			envBranch = strings.Replace(env, pf.source+"_", "", 1)
		}
		if strings.HasPrefix(pf.moduleDir, "/") {
			moduleDir = pf.moduleDir
		}
		if force {
			createOrPurgeDir(moduleDir, "resolvePuppetfile()")
			moduleDir = checkDirAndCreate(moduleDir, "moduleDir for source "+pf.source)
		} else {
			moduleDir = checkDirAndCreate(moduleDir, "moduleDir for "+pf.source)
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
				targetDir := moduleDir + gitName + "/"
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
					targetDir = basedir + "/" + gitModule.installPath + "/" + gitName + "/"
				}

				success := false
				moduleCacheDir := config.ModulesCacheDir + strings.Replace(strings.Replace(gitModule.git, "/", "_", -1), ":", "-", -1)

				if gitModule.link {
					Debugf("Trying to resolve " + moduleCacheDir + " with branch " + tree)
					success = syncToModuleDir(moduleCacheDir, targetDir, tree, true, gitModule.ignoreUnreachable)
				}

				if len(gitModule.fallback) > 0 {
					Debugf("jjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjj")
					if !success {
						for i, fallbackBranch := range gitModule.fallback {
							if i == len(gitModule.fallback)-1 {
								// last try
								gitModule.ignoreUnreachable = true
							}
							Debugf("Trying to resolve " + moduleCacheDir + " with branch " + fallbackBranch)
							success = syncToModuleDir(moduleCacheDir, targetDir, fallbackBranch, true, gitModule.ignoreUnreachable)
							if success {
								break
							}
						}
					}
				} else {
					success = syncToModuleDir(moduleCacheDir, targetDir, tree, gitModule.ignoreUnreachable, gitModule.ignoreUnreachable)
				}

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
	if len(exisitingModuleDirs) > 0 && len(moduleParam) == 0 {
		for d := range exisitingModuleDirs {
			Debugf("Removing unmanaged file " + d)
			if err := os.RemoveAll(d); err != nil {
				Debugf("Error while trying to remove unmanaged file " + d)
			}
		}
	}
	if !debug && !verbose && !info && !quiet {
		uiprogress.Stop()
	}
}
