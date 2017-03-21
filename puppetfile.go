package main

import (
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/henvic/uiprogress"
)

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
			Debugf("Puppet environment: " + source + " (remote=" + sa.Remote + ", basedir=" + sa.Basedir + ", private_key=" + sa.PrivateKey + ", prefix=" + sa.Prefix + ")")
			if len(sa.PrivateKey) > 0 {
				if _, err := os.Stat(sa.PrivateKey); err != nil {
					Fatalf("resolvePuppetEnvironment(): could not find SSH private key " + sa.PrivateKey + "error: " + err.Error())
				}
			}

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

				for k, branch := range branches {
					branch = strings.TrimLeft(branch, "* ")
					// XXX: maybe make this user configurable (either with dedicated file or as YAML array in g10k config)
					if strings.Contains(branch, ";") || strings.Contains(branch, "&") || strings.Contains(branch, "|") || strings.HasPrefix(branch, "tmp/") && strings.HasSuffix(branch, "/head") || (len(envBranch) > 0 && branch != envBranch) {
						Debugf("Skipping branch " + branch)
						if k == len(branches)-1 {
							Warnf("WARNING: Couldn't find specified branch '" + envBranch + "' anywhere in source '" + source + "' (" + sa.Remote + ")")
						}
						continue
					}

					wg.Add(1)

					go func(branch string) {
						defer wg.Done()
						if len(branch) != 0 {
							Debugf("Resolving branch: " + branch)

							targetDir := sa.Basedir + sa.Prefix + "_" + strings.Replace(branch, "/", "_", -1) + "/"
							if sa.Prefix == "false" {
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
		for _, gitModule := range pf.gitModules {
			gitModule.privateKey = pf.privateKey
			if _, ok := uniqueGitModules[gitModule.git]; !ok {
				uniqueGitModules[gitModule.git] = gitModule
			}
		}
		for forgeModuleName, fm := range pf.forgeModules {
			//fmt.Println("Found Forge module ", forgeModuleName, " with version", fm.version)
			fm.baseUrl = pf.forgeBaseURL
			fm.cacheTtl = pf.forgeCacheTtl
			forgeModuleName = strings.Replace(forgeModuleName, "/", "-", -1)
			if _, ok := uniqueForgeModules[forgeModuleName+"-"+fm.version]; !ok {
				uniqueForgeModules[forgeModuleName+"-"+fm.version] = fm
			}
		}
	}
	if !debug && !verbose && !info {
		uiprogress.Start()
	}
	//fmt.Println(uniqueGitModules)
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
				} else if gitModule.link {
					if pfMode {
						if len(os.Getenv("g10k_branch")) > 0 {
							tree = os.Getenv("g10k_branch")
						} else {
							Fatalf("resolvePuppetfile(): found module " + gitName + " with module link mode enabled and g10k in Puppetfile mode which is not supported, as I can not detect the environment branch of the Puppetfile. You can explicitly set the module link branch you want to use in Puppetfile mode by setting the environment variable 'g10k_branch'")
						}
					} else {
						tree = envBranch
					}
				}
				success := false
				//fmt.Println(gitModule.fallback)
				moduleCacheDir := config.ModulesCacheDir + strings.Replace(strings.Replace(gitModule.git, "/", "_", -1), ":", "-", -1)
				if len(gitModule.fallback) > 0 {
					success = syncToModuleDir(moduleCacheDir, targetDir, tree, true, gitModule.ignoreUnreachable)
					if !success {
						for i, fallbackBranch := range gitModule.fallback {
							if i == len(gitModule.fallback)-1 {
								// last try
								gitModule.ignoreUnreachable = true
							}
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
	if len(exisitingModuleDirs) > 0 {
		for d := range exisitingModuleDirs {
			Debugf("Removing unmanaged file " + d)
			if err := os.RemoveAll(d); err != nil {
				Debugf("Error while trying to remove unmanaged file " + d)
			}
		}
	}
	if !debug && !verbose && !info {
		uiprogress.Stop()
	}
}
