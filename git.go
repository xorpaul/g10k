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

func resolveGitRepositories(uniqueGitModules map[string]string) {
	var wgGit sync.WaitGroup
	for url, sshPrivateKey := range uniqueGitModules {
		wgGit.Add(1)
		go func(url string, sshPrivateKey string) {
			defer wgGit.Done()
			if len(sshPrivateKey) > 0 {
				Debugf("git repo url " + url + " with ssh key " + sshPrivateKey)
			} else {
				Debugf("git repo url " + url + " without ssh key")
			}

			// create save directory name from Git repo name
			repoDir := strings.Replace(strings.Replace(url, "/", "_", -1), ":", "-", -1)
			workDir := config.ModulesCacheDir + repoDir

			doMirrorOrUpdate(url, workDir, sshPrivateKey, false)
			//	doCloneOrPull(source, workDir, targetDir, sa.Remote, branch, sa.PrivateKey)

		}(url, sshPrivateKey)
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

	needSshKey := true
	if strings.Contains(url, "github.com") || len(sshPrivateKey) == 0 {
		needSshKey = false
	} else {
		needSshKey = true
		//doCheckout = compareGitVersions(workDir, url, branch)
	}

	er := ExecResult{}
	gitCmd := "git clone --mirror " + url + " " + workDir
	if dirExists {
		gitCmd = "git --git-dir " + workDir + " remote update --prune"
	}

	if needSshKey {
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

func syncToModuleDir(srcDir string, targetDir string, tree string) {
	mutex.Lock()
	syncGitCount++
	mutex.Unlock()
	logCmd := "git --git-dir " + srcDir + " log -n1 --pretty=format:%H " + tree
	er := executeCommand(logCmd, config.Timeout, false)
	hashFile := targetDir + "/.latest_commit"
	needToSync := true
	if len(er.output) > 0 {
		targetHash, _ := ioutil.ReadFile(hashFile)
		if string(targetHash) == er.output {
			needToSync = false
			//Debugf("syncToModuleDir(): Skipping, because no diff found between " + srcDir + "(" + er.output + ") and " + targetDir + "(" + string(targetHash) + ")")
		}

	}
	if needToSync {
		Infof("Need to sync " + targetDir)
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
			log.Println("syncToModuleDir(): Failed to execute command: ", cmd, " Output: ", string(out))
			os.Exit(1)
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
