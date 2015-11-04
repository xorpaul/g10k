package main

import (
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
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
							targetDir := sa.Basedir + source + "_" + strings.Replace(branch, "/", "_", -1) + "/"
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
