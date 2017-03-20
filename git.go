package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xorpaul/uiprogress"
)

func resolveGitRepositories(uniqueGitModules map[string]GitModule) {
	if len(uniqueGitModules) <= 0 {
		Debugf("uniqueGitModules[] is empty, skipping...")
		return
	}
	var wgGit sync.WaitGroup
	bar := uiprogress.AddBar(len(uniqueGitModules)).AppendCompleted().PrependElapsed()
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return fmt.Sprintf("Resolving Git modules (%d/%d)", b.Current(), len(uniqueGitModules))
	})
	for url, gm := range uniqueGitModules {
		wgGit.Add(1)
		privateKey := gm.privateKey
		go func(url string, privateKey string, gm GitModule) {
			defer wgGit.Done()
			defer bar.Incr()
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

		}(url, privateKey, gm)
	}
	wgGit.Wait()
}

func doMirrorOrUpdate(url string, workDir string, sshPrivateKey string, allowFail bool) bool {
	needSSHKey := true
	if strings.Contains(url, "github.com") || len(sshPrivateKey) == 0 {
		needSSHKey = false
	}

	er := ExecResult{}
	gitCmd := "git clone --mirror " + url + " " + workDir
	if fileExists(workDir) {
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

func syncToModuleDir(srcDir string, targetDir string, tree string, allowFail bool, ignoreUnreachable bool) bool {
	mutex.Lock()
	syncGitCount++
	mutex.Unlock()
	logCmd := "git --git-dir " + srcDir + " log -n1 --pretty=format:%H " + tree
	er := executeCommand(logCmd, config.Timeout, allowFail)
	hashFile := targetDir + "/.latest_commit"
	needToSync := true
	if er.returnCode != 0 && allowFail {
		if ignoreUnreachable {
			Infof("Failed to populate module " + targetDir + " but ignore-unreachable is set. Continuing...")
			purgeDir(targetDir, "syncToModuleDir, because ignore-unreachable is set for this module")
		}
		return false
	}

	if len(er.output) > 0 {
		targetHash, _ := ioutil.ReadFile(hashFile)
		if string(targetHash) == er.output {
			needToSync = false
			//Debugf("Skipping, because no diff found between " + srcDir + "(" + er.output + ") and " + targetDir + "(" + string(targetHash) + ")")
		}

	}
	if needToSync && er.returnCode == 0 {
		Infof("Need to sync " + targetDir)
		mutex.Lock()
		needSyncGitCount++
		mutex.Unlock()
		if !dryRun {
			createOrPurgeDir(targetDir, "syncToModuleDir()")
			gitArchiveArgs := []string{"--git-dir", srcDir, "archive", tree}
			cmd := exec.Command("git", gitArchiveArgs...)
			Debugf("Executing git --git-dir " + srcDir + " archive " + tree)
			cmdOut, err := cmd.StdoutPipe()
			if err != nil {
				if !allowFail {
					Infof("Failed to populate module " + targetDir + " but ignore-unreachable is set. Continuing...")
				} else {
					return false
				}
				Fatalf("syncToModuleDir(): Failed to execute command: git --git-dir " + srcDir + " archive " + tree + " Error: " + err.Error())
			}
			cmd.Start()

			before := time.Now()
			unTar(cmdOut, targetDir)
			duration := time.Since(before).Seconds()
			mutex.Lock()
			ioGitTime += duration
			mutex.Unlock()
			Verbosef("syncToModuleDir(): Executing git --git-dir " + srcDir + " archive " + tree + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")

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
	return true
}
