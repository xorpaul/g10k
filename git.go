package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

func resolveGitRepositories(uniqueGitModules map[string]GitModule) {
	var wgGit sync.WaitGroup
	for url, gm := range uniqueGitModules {
		wgGit.Add(1)
		privateKey := gm.privateKey
		go func(url string, privateKey string) {
			defer wgGit.Done()
			if len(gm.privateKey) > 0 {
				Debugf("git repo url " + url + " with ssh key " + privateKey)
			} else {
				Debugf("git repo url " + url + " without ssh key")
			}

			//log.Println(config)
			// create save directory name from Git repo name
			repoDir := strings.Replace(strings.Replace(url, "/", "_", -1), ":", "-", -1)
			workDir := config.ModulesCacheDir + repoDir

			doMirrorOrUpdate(url, workDir, privateKey, gm.ignoreUnreachable)
			//	doCloneOrPull(source, workDir, targetDir, sa.Remote, branch, sa.PrivateKey)

		}(url, privateKey)
	}
	wgGit.Wait()
}

func doMirrorOrUpdate(url string, workDir string, sshPrivateKey string, allowFail bool) bool {
	dirExists := false
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		dirExists = false
	} else {
		dirExists = true
		//doCheckout = compareGitVersions(workDir, url, branch)
	}

	needSSHKey := true
	if strings.Contains(url, "github.com") || len(sshPrivateKey) == 0 {
		needSSHKey = false
	} else {
		needSSHKey = true
		//doCheckout = compareGitVersions(workDir, url, branch)
	}

	er := ExecResult{}
	gitCmd := "git clone --mirror " + url + " " + workDir
	if dirExists {
		gitCmd = "git --git-dir " + workDir + " remote update --prune"
	}

	if needSSHKey {
		er = executeCommand("ssh-agent bash -c 'ssh-add "+sshPrivateKey+"; "+gitCmd+"'", config.Timeout, allowFail)
	} else {
		er = executeCommand(gitCmd, config.Timeout, allowFail)
	}

	if er.returnCode != 0 {
		fmt.Println("WARN: git repository " + url + " does not exist or is unreachable at this moment!")
		return false
	}
	return true
}

func syncToModuleDir(srcDir string, targetDir string, tree string, allowFail bool) {
	mutex.Lock()
	syncGitCount++
	mutex.Unlock()
	logCmd := "git --git-dir " + srcDir + " log -n1 --pretty=format:%H " + tree
	er := executeCommand(logCmd, config.Timeout, allowFail)
	hashFile := targetDir + "/.latest_commit"
	needToSync := true
	if er.returnCode != 0 && allowFail {
		Infof("Failed to populate module " + targetDir + " but ignore-unreachable is set. Continuing...")
		return
	}

	if len(er.output) > 0 {
		targetHash, _ := ioutil.ReadFile(hashFile)
		if string(targetHash) == er.output {
			needToSync = false
			//Debugf("syncToModuleDir(): Skipping, because no diff found between " + srcDir + "(" + er.output + ") and " + targetDir + "(" + string(targetHash) + ")")
		}

	}
	if needToSync && er.returnCode == 0 {
		Infof("Need to sync " + targetDir)
		mutex.Lock()
		needSyncGitCount++
		mutex.Unlock()
		if !dryRun {
			createOrPurgeDir(targetDir, "syncToModuleDir()")
			cmd := "git --git-dir " + srcDir + " archive " + tree + " | tar -x -C " + targetDir
			before := time.Now()
			out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			duration := time.Since(before).Seconds()
			mutex.Lock()
			cpGitTime += duration
			mutex.Unlock()
			Verbosef("syncToModuleDir(): Executing " + cmd + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
			if err != nil {
				if !allowFail {
					log.Println("syncToModuleDir(): Failed to execute command: ", cmd, " Output: ", string(out))
					os.Exit(1)
				} else {
					Infof("Failed to populate module " + targetDir + " but ignore-unreachable is set. Continuing...")
					return
				}
			}

			er = executeCommand(logCmd, config.Timeout, false)
			if len(er.output) > 0 {
				Debugf("Writing hash " + er.output + " from command " + logCmd + " to " + hashFile)
				f, _ := os.Create(hashFile)
				defer f.Close()
				f.WriteString(er.output)
				f.Sync()
			}
		}
	}
}
