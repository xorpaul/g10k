package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

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

func resolvePuppetEnvironment(tags bool, outputNameTag string) {
	wg := sizedwaitgroup.New(config.MaxExtractworker + 1)
	allPuppetfiles := make(map[string]Puppetfile)
	allEnvironments := make(map[string]bool)
	allBasedirs := make(map[string]bool)
	foundMatch := false
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

			workDir := filepath.Join(config.EnvCacheDir, source+".git")
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
						Debugf("Skipping branch " + branch + " of source " + source + ", because of invalid character(s) inside the branch name")
						continue
					}
					mutex.Lock()
					allEnvironments[prefix+branch] = true
					mutex.Unlock()
					if len(branchParam) > 0 {
						if branch == branchParam {
							foundBranch = true
						} else {
							Debugf("Environment " + prefix + branch + " of source " + source + " does not match branch name filter '" + branchParam + "', skipping")
							continue
						}
					} else if len(environmentParam) > 0 {
						if source+"_"+branch == environmentParam {
							foundMatch = true
						} else {
							Debugf("Environment " + prefix + branch + " of source " + source + " does not match environment name filter '" + environmentParam + "', skipping")
							continue
						}
					}

					wg.Add()

					go func(branch string, sa Source, prefix string) {
						defer wg.Done()
						if len(branch) != 0 {
							Debugf("Resolving environment " + prefix + branch + " of source " + source)

							renamedBranch := branch
							if (len(outputNameTag) > 0) && (len(branchParam) > 0) {
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

							targetDir := filepath.Join(sa.Basedir, prefix+strings.Replace(renamedBranch, "/", "_", -1))
							targetDir = normalizeDir(targetDir)

							env := strings.Replace(strings.Replace(targetDir, sa.Basedir, "", 1), "/", "", -1)
							syncToModuleDir(workDir, targetDir, branch, false, false, env)
							pf := filepath.Join(targetDir, "Puppetfile")
							deployFile := filepath.Join(targetDir, ".g10k-deploy.json")
							if !fileExists(pf) {
								Debugf("Skipping branch " + source + "_" + branch + " because " + targetDir + "Puppetfile does not exist")
							} else {
								if fileExists(deployFile) {
									pfHashSum := getSha256sumFile(pf)
									dr := readDeployResultFile(deployFile)
									if pfHashSum == dr.PuppetfileChecksum && dr.DeploySuccess {
										Infof("Skipping Puppetfile sync of branch " + source + "_" + branch + " because " + pf + " did not change")
										dr.FinishedAt = time.Now()
										writeStructJSONFile(deployFile, dr)
									}
								}
								puppetfile := readPuppetfile(pf, sa.PrivateKey, source, sa.ForceForgeVersions, false)
								puppetfile.workDir = normalizeDir(targetDir)
								puppetfile.controlRepoBranch = branch
								mutex.Lock()
								for _, moduleDir := range puppetfile.moduleDirs {
									desiredContent = append(desiredContent, filepath.Join(puppetfile.workDir, moduleDir))
									checkDirAndCreate(filepath.Join(puppetfile.workDir, moduleDir), "moduledir for env")
								}
								allPuppetfiles[env] = puppetfile
								allBasedirs[sa.Basedir] = true
								mutex.Unlock()

							}
						}
					}(branch, sa, prefix)
				}

				if sa.WarnMissingBranch && !foundBranch {
					Warnf("WARNING: Couldn't find specified branch '" + branchParam + "' anywhere in source '" + source + "' (" + sa.Remote + ")")
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
	if len(environmentParam) > 0 {
		if !foundMatch {
			Warnf("WARNING: Environment '" + environmentParam + "' cannot be found in any source and will not be deployed.")
		}
	}
	//fmt.Println("allPuppetfiles: ", allPuppetfiles, len(allPuppetfiles))
	//fmt.Println("allPuppetfiles[0]: ", allPuppetfiles["postinstall"])
	resolvePuppetfile(allPuppetfiles)
	//fmt.Printf("%+v\n", allEnvironments)
	purgeUnmanagedContent(allBasedirs, allEnvironments)
}

// resolveSourcePrefix implements the prefix read out from each source given in the config file, like r10k https://github.com/puppetlabs/r10k/blob/master/doc/dynamic-environments/configuration.mkd#prefix
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
		Debugf("Resolving branch " + env + " of source " + pf.source)
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
		// this prevents g10k from purging module directories on the subsequent run in -puppetfile mode
		basedir := ""
		if !pfMode {
			basedir = checkDirAndCreate(pf.workDir, "basedir 2 for source "+pf.source)
		}

		for _, moduleDir := range pf.moduleDirs {
			moduleDir = normalizeDir(filepath.Join(pf.workDir, moduleDir))
			exisitingModuleDirsFI, _ := ioutil.ReadDir(moduleDir)
			mutex.Lock()
			for _, exisitingModuleDir := range exisitingModuleDirsFI {
				//fmt.Println("adding dir: ", moduleDir+exisitingModuleDir.Name())
				exisitingModuleDirs[filepath.Join(moduleDir, exisitingModuleDir.Name())] = empty
			}
			mutex.Unlock()
		}

		for gitName, gitModule := range pf.gitModules {
			moduleDir := filepath.Join(pf.workDir, gitModule.moduleDir)
			moduleDir = normalizeDir(moduleDir)
			if gitModule.local {
				moduleDirectory := filepath.Join(moduleDir, gitName)
				Debugf("Not deleting " + moduleDirectory + " as it is declared as a local module")
				// remove this module from the exisitingModuleDirs map
				if len(gitModule.installPath) > 0 {
					moduleDirectory = filepath.Join(normalizeDir(basedir), normalizeDir(gitModule.installPath), gitName)
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
				targetDir := normalizeDir(filepath.Join(moduleDir, gitName))
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
						// we want only the branch name of the control repo and not the resulting
						// Puppet environment folder name, which could contain a prefix
						tree = pf.controlRepoBranch
					}
				}

				if len(gitModule.installPath) > 0 {
					targetDir = filepath.Join(basedir, normalizeDir(gitModule.installPath), gitName)
				}
				targetDir = normalizeDir(targetDir)
				success := false
				moduleCacheDir := filepath.Join(config.ModulesCacheDir, strings.Replace(strings.Replace(gitModule.git, "/", "_", -1), ":", "-", -1))

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
				moduleDirectory := filepath.Join(moduleDir, gitName)
				if len(gitModule.installPath) > 0 {
					moduleDirectory = filepath.Join(normalizeDir(basedir), normalizeDir(gitModule.installPath), gitName)
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
			moduleDir := filepath.Join(pf.workDir, fm.moduleDir)
			moduleDir = normalizeDir(moduleDir)
			go func(forgeModuleName string, fm ForgeModule, moduleDir string, env string) {
				defer wg.Done()
				syncForgeToModuleDir(forgeModuleName, fm, moduleDir, env)
				// remove this module from the exisitingModuleDirs map
				mutex.Lock()
				mDir := filepath.Join(moduleDir, fm.name)
				if _, ok := exisitingModuleDirs[mDir]; ok {
					delete(exisitingModuleDirs, mDir)
				}
				mutex.Unlock()
			}(forgeModuleName, fm, moduleDir, env)
		}
	}
	wg.Wait()

	if stringSliceContains(config.PurgeLevels, "puppetfile") {
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

	for _, pf := range allPuppetfiles {
		deployFile := filepath.Join(pf.workDir, ".g10k-deploy.json")
		if fileExists(deployFile) {
			Debugf("Finishing writing to deploy file " + deployFile)
			dr := readDeployResultFile(deployFile)
			dr.DeploySuccess = true
			dr.FinishedAt = time.Now()
			dr.PuppetfileChecksum = getSha256sumFile(filepath.Join(pf.workDir, "Puppetfile"))
			writeStructJSONFile(deployFile, dr)
			mutex.Lock()
			desiredContent = append(desiredContent, deployFile)
			mutex.Unlock()
		}
	}

}
