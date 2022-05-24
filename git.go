package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xorpaul/uiprogress"
)

func resolveGitRepositories(uniqueGitModules map[string]GitModule) {
	defer timeTrack(time.Now(), funcName())
	if len(uniqueGitModules) <= 0 {
		Debugf("uniqueGitModules[] is empty, skipping...")
		return
	}
	bar := uiprogress.AddBar(len(uniqueGitModules)).AppendCompleted().PrependElapsed()
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return fmt.Sprintf("Resolving Git modules (%d/%d)", b.Current(), len(uniqueGitModules))
	})
	// Dummy channel to coordinate the number of concurrent goroutines.
	// This channel should be buffered otherwise we will be immediately blocked
	// when trying to fill it.

	Debugf("Resolving " + strconv.Itoa(len(uniqueGitModules)) + " Git modules with " + strconv.Itoa(config.Maxworker) + " workers")
	concurrentGoroutines := make(chan struct{}, config.Maxworker)
	// Fill the dummy channel with config.Maxworker empty struct.
	for i := 0; i < config.Maxworker; i++ {
		concurrentGoroutines <- struct{}{}
	}

	// The done channel indicates when a single goroutine has finished its job.
	done := make(chan bool)
	// The waitForAllJobs channel allows the main program
	// to wait until we have indeed done all the jobs.
	waitForAllJobs := make(chan bool)
	// Collect all the jobs, and since the job is finished, we can
	// release another spot for a goroutine.
	go func() {
		for _, gm := range uniqueGitModules {
			go func(gm GitModule) {
				<-done
				// Say that another goroutine can now start.
				concurrentGoroutines <- struct{}{}
			}(gm)
		}
		// We have collected all the jobs, the program can now terminate
		waitForAllJobs <- true
	}()
	wg := sync.WaitGroup{}
	wg.Add(len(uniqueGitModules))

	for url, gm := range uniqueGitModules {
		privateKey := gm.privateKey
		go func(url string, gm GitModule, bar *uiprogress.Bar) {
			// Try to receive from the concurrentGoroutines channel. When we have something,
			// it means we can start a new goroutine because another one finished.
			// Otherwise, it will block the execution until an execution
			// spot is available.
			<-concurrentGoroutines
			defer bar.Incr()
			defer wg.Done()

			if gm.useSSHAgent {
				Debugf("git repo url " + url + " with loaded SSH keys from ssh-agent")
			} else if len(gm.privateKey) > 0 {
				Debugf("git repo url " + url + " with SSH key " + privateKey)
			} else {
				Debugf("git repo url " + url + " without ssh key")
			}

			//log.Println(config)
			// create save directory name from Git repo name
			repoDir := strings.Replace(strings.Replace(url, "/", "_", -1), ":", "-", -1)
			workDir := filepath.Join(config.ModulesCacheDir, repoDir)

			success := doMirrorOrUpdate(gm, workDir, 0)
			if !success && !config.UseCacheFallback {
				Fatalf("Fatal: Failed to clone or pull " + url + " to " + workDir)
			}
			done <- true
		}(url, gm, bar)
	}

	// Wait for all jobs to finish
	<-waitForAllJobs
	wg.Wait()
}

func doMirrorOrUpdate(gitModule GitModule, workDir string, retryCount int) bool {
	//fmt.Printf("%+v\n", gitModule)
	isControlRepo := strings.HasPrefix(workDir, config.EnvCacheDir)
	isInModulesCacheDir := strings.HasPrefix(workDir, config.ModulesCacheDir)

	explicitlyLoadSSHKey := true
	if len(gitModule.privateKey) == 0 || strings.Contains(gitModule.git, "github.com") || gitModule.useSSHAgent {
		if gitModule.useSSHAgent {
			explicitlyLoadSSHKey = false
		} else if isControlRepo {
			explicitlyLoadSSHKey = true
		} else {
			explicitlyLoadSSHKey = false
		}
	}
	er := ExecResult{}
	gitCmd := "git clone --mirror " + gitModule.git + " " + workDir
	if config.CloneGitModules && !isControlRepo && !isInModulesCacheDir {
		gitCmd = "git clone --single-branch --branch " + gitModule.tree + " " + gitModule.git + " " + workDir
	}
	if isDir(workDir) {
		if detectGitRemoteUrlChange(workDir, gitModule.git) {
			purgeDir(workDir, "git remote url changed")
		} else {
			gitCmd = "git --git-dir " + workDir + " remote update --prune"
		}
	}

	if explicitlyLoadSSHKey {
		sshAddCmd := "ssh-add "
		if runtime.GOOS == "darwin" {
			sshAddCmd = "ssh-add -K "
		}
		er = executeCommand("ssh-agent bash -c '"+sshAddCmd+gitModule.privateKey+"; "+gitCmd+"'", config.Timeout, gitModule.ignoreUnreachable)
	} else {
		er = executeCommand(gitCmd, config.Timeout, gitModule.ignoreUnreachable)
	}

	if er.returnCode != 0 {
		if config.UseCacheFallback {
			Warnf("WARN: git repository " + gitModule.git + " does not exist or is unreachable at this moment!")
			Warnf("WARN: Trying to use cache for " + gitModule.git + " git repository")
			return false
		} else if config.RetryGitCommands && retryCount > -1 {
			Warnf("WARN: git command failed: " + gitCmd + " deleting local cached repository and retrying...")
			purgeDir(workDir, "doMirrorOrUpdate, because git command failed, retrying")
			return doMirrorOrUpdate(gitModule, workDir, retryCount-1)
		}
		Warnf("WARN: git repository " + gitModule.git + " does not exist or is unreachable at this moment! Error: " + er.output)
		return false
	}
	return true
}

func syncToModuleDir(gitModule GitModule, srcDir string, targetDir string, correspondingPuppetEnvironment string) bool {
	startedAt := time.Now()
	mutex.Lock()
	syncGitCount++
	mutex.Unlock()
	if !isDir(srcDir) {
		if config.UseCacheFallback {
			Fatalf("Could not find cached git module " + srcDir)
		}
	}
	revParseCmd := "git --git-dir " + srcDir + " rev-parse --verify '" + gitModule.tree
	if !config.GitObjectSyntaxNotSupported {
		revParseCmd = revParseCmd + "^{object}'"
	} else {
		revParseCmd = revParseCmd + "'"
	}

	isControlRepo := strings.HasPrefix(srcDir, config.EnvCacheDir)

	er := executeCommand(revParseCmd, config.Timeout, gitModule.ignoreUnreachable)
	hashFile := filepath.Join(targetDir, ".latest_commit")
	deployFile := filepath.Join(targetDir, ".g10k-deploy.json")
	needToSync := true
	if er.returnCode != 0 {
		if gitModule.ignoreUnreachable {
			Debugf("Failed to populate module " + targetDir + " but ignore-unreachable is set. Continuing...")
			purgeDir(targetDir, "syncToModuleDir, because ignore-unreachable is set for this module")
		}
		return false
	}

	if len(er.output) > 0 {
		commitHash := strings.TrimSuffix(er.output, "\n")
		if strings.HasPrefix(srcDir, config.EnvCacheDir) {
			mutex.Lock()
			desiredContent = append(desiredContent, deployFile)
			mutex.Unlock()
			if fileExists(deployFile) {
				dr := readDeployResultFile(deployFile)
				if dr.Signature == strings.TrimSuffix(er.output, "\n") {
					needToSync = false
					// need to get the content of the git repository to detect and purge unmanaged files
					addDesiredContent(srcDir, gitModule.tree, targetDir)
				}
			}
		} else {
			Debugf("adding path to managed content: " + targetDir)
			mutex.Lock()
			desiredContent = append(desiredContent, hashFile)
			desiredContent = append(desiredContent, targetDir)
			mutex.Unlock()
			targetHashByte, _ := ioutil.ReadFile(hashFile)
			targetHash := string(targetHashByte)
			Debugf("string content of " + hashFile + " is: " + targetHash)
			if targetHash == commitHash {
				needToSync = false
				mutex.Lock()
				unchangedModuleDirs = append(unchangedModuleDirs, targetDir)
				mutex.Unlock()
				Debugf("Skipping, because no diff found between " + srcDir + "(" + commitHash + ") and " + targetDir + "(" + targetHash + ")")
			} else {
				Debugf("Need to sync, because existing Git module: " + targetDir + " has commit " + targetHash + " and the to be synced commit is: " + commitHash)
			}
		}

	}
	if needToSync && er.returnCode == 0 {
		Infof("Need to sync " + targetDir)
		mutex.Lock()
		needSyncDirs = append(needSyncDirs, targetDir)
		if _, ok := needSyncEnvs[correspondingPuppetEnvironment]; !ok {
			needSyncEnvs[correspondingPuppetEnvironment] = empty
		}
		needSyncGitCount++
		mutex.Unlock()

		if !dryRun && !config.CloneGitModules || isControlRepo {
			if pfMode {
				purgeDir(targetDir, "git dir with changes in -puppetfile mode")
			}
			checkDirAndCreate(targetDir, "git dir")
			gitArchiveArgs := []string{"--git-dir", srcDir, "archive", gitModule.tree}
			cmd := exec.Command("git", gitArchiveArgs...)
			Debugf("Executing git --git-dir " + srcDir + " archive " + gitModule.tree)
			cmdOut, err := cmd.StdoutPipe()
			if err != nil {
				if !gitModule.ignoreUnreachable {
					Infof("Failed to populate module " + targetDir + " but ignore-unreachable is set. Continuing...")
				} else {
					return false
				}
				Fatalf("syncToModuleDir(): Failed to execute command: git --git-dir " + srcDir + " archive " + gitModule.tree + " Error: " + err.Error())
			}
			cmd.Start()

			before := time.Now()
			unTar(cmdOut, targetDir)
			duration := time.Since(before).Seconds()
			mutex.Lock()
			ioGitTime += duration
			mutex.Unlock()

			err = cmd.Wait()
			if err != nil {
				Fatalf("syncToModuleDir(): Failed to execute command: git --git-dir " + srcDir + " archive " + gitModule.tree + " Error: " + err.Error())
				//"\nIf you are using GitLab please ensure that you've added your deploy key to your repository." +
				//"\nThe Puppet environment which is using this unresolveable repository is " + correspondingPuppetEnvironment)
			}

			Verbosef("syncToModuleDir(): Executing git --git-dir " + srcDir + " archive " + gitModule.tree + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")

			commitHash := strings.TrimSuffix(er.output, "\n")
			if isControlRepo {
				Debugf("Writing to deploy file " + deployFile)
				dr := DeployResult{
					Name:      gitModule.tree,
					Signature: commitHash,
					StartedAt: startedAt,
				}
				writeStructJSONFile(deployFile, dr)
			} else {
				Debugf("Writing hash " + commitHash + " from command " + revParseCmd + " to " + hashFile)
				f, _ := os.Create(hashFile)
				defer f.Close()
				f.WriteString(commitHash)
				f.Sync()
			}

		} else if config.CloneGitModules {
			return doMirrorOrUpdate(gitModule, targetDir, 0)
		}
	}
	return true
}

// addDesiredContent takes the given git repository directory and the
// relevant reference (branch, commit hash, tag) and adds its content to
// the global desiredContent slice so that it doesn't get purged by g10k
func addDesiredContent(gitDir string, tree string, targetDir string) {
	treeCmd := "git --git-dir " + gitDir + " ls-tree --full-tree -r -t --name-only " + tree
	er := executeCommand(treeCmd, config.Timeout, false)
	foundGitFiles := strings.Split(er.output, "\n")
	mutex.Lock()
	for _, desiredFile := range foundGitFiles[:len(foundGitFiles)-1] {
		desiredContent = append(desiredContent, filepath.Join(targetDir, desiredFile))

		// because we're using -r which prints git managed files in subfolders like this: foo/test3
		// we have to split up the given string and add the possible parent directories (foo in this case)
		parentDirs := strings.Split(desiredFile, "/")
		if len(parentDirs) > 1 {
			for _, dir := range parentDirs[:len(parentDirs)-1] {
				desiredContent = append(desiredContent, filepath.Join(targetDir, dir))
			}
		}
	}
	mutex.Unlock()

}

func detectDefaultBranch(gitDir string) string {
	remoteShowOriginCmd := "git ls-remote --symref " + gitDir
	er := executeCommand(remoteShowOriginCmd, config.Timeout, false)
	foundRefs := strings.Split(er.output, "\n")
	if len(foundRefs) < 1 {
		Fatalf("Unable to detect default branch for git repository with command git ls-remote --symref " + gitDir)
	}
	// should look like this:
	// ref: refs/heads/main\tHEAD
	headBranchParts := strings.Split(foundRefs[0], "\t")
	defaultBranch := strings.TrimPrefix(string(headBranchParts[0]), "ref: refs/heads/")
	//fmt.Println(defaultBranch)
	return defaultBranch
}

func detectGitRemoteUrlChange(d string, url string) bool {
	gitRemoteCmd := "git --git-dir " + d + " remote -v"

	er := executeCommand(gitRemoteCmd, config.Timeout, false)
	if er.returnCode != 0 {
		Warnf("WARN: Could not detect remote URL for git repository " + d + " trying to purge it and mirror is again")
		return true
	}

	f := strings.Fields(er.output)
	if len(f) < 3 {
		Warnf("WARN: Could not detect remote URL for git repository " + d + " trying to purge it and mirror is again")
		return true
	}
	configuredRemote := f[1]
	return configuredRemote != url
}
