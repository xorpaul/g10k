package main

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/kballard/go-shellquote"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Debugf is a helper function for debug logging if global variable debug is set to true
func Debugf(s string) {
	if debug != false {
		log.Print("DEBUG " + fmt.Sprint(s))
	}
}

// Verbosef is a helper function for debug logging if global variable verbose is set to true
func Verbosef(s string) {
	if debug != false || verbose != false {
		log.Print(fmt.Sprint(s))
	}
}

// Infof is a helper function for debug logging if global variable info is set to true
func Infof(s string) {
	if debug != false || verbose != false || info != false {
		color.Set(color.FgGreen)
		fmt.Println(s)
		color.Unset()
	}
}

// fileExists checks if the given file exists and return a bool
func fileExists(file string) bool {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	}
	return true
}

// checkDirAndCreate tests if the given directory exists and tries to create it
func checkDirAndCreate(dir string, name string) string {
	if !dryRun {
		if len(dir) != 0 {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				//log.Printf("checkDirAndCreate(): trying to create dir '%s' as %s", dir, name)
				if err := os.MkdirAll(dir, 0777); err != nil {
					log.Print("checkDirAndCreate(): Error: failed to create directory: ", dir)
					os.Exit(1)
				}
			}
		} else {
			// TODO make dir optional
			log.Print("dir setting '" + name + "' missing! Exiting!")
			os.Exit(1)
		}
	}
	if !strings.HasSuffix(dir, "/") {
		dir = dir + "/"
	}
	Debugf("Using as " + name + ": " + dir)
	return dir
}

func createOrPurgeDir(dir string, callingFunction string) {
	if !dryRun {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			Debugf("createOrPurgeDir(): Trying to create dir: " + dir + " called from " + callingFunction)
			os.Mkdir(dir, 0777)
		} else {
			Debugf("createOrPurgeDir(): Trying to remove: " + dir + " called from " + callingFunction)
			if err := os.RemoveAll(dir); err != nil {
				log.Print("createOrPurgeDir(): error: removing dir failed", err)
			}
			Debugf("createOrPurgeDir(): Trying to create dir: " + dir + " called from " + callingFunction)
			os.Mkdir(dir, 0777)
		}
	}
}

func purgeDir(dir string, callingFunction string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		Debugf("purgeDir(): Unnecessary to remove dir: " + dir + " it does not exist. Called from " + callingFunction)
	} else {
		Debugf("purgeDir(): Trying to remove: " + dir + " called from " + callingFunction)
		if err := os.RemoveAll(dir); err != nil {
			log.Print("purgeDir(): os.RemoveAll() error: removing dir failed: ", err)
			if err = syscall.Unlink(dir); err != nil {
				log.Print("purgeDir(): syscall.Unlink() error: removing link failed: ", err)
			}
		}
	}
}

func executeCommand(command string, timeout int, allowFail bool) ExecResult {
	Debugf("Executing " + command)
	parts := strings.SplitN(command, " ", 2)
	cmd := parts[0]
	cmdArgs := []string{}
	if len(parts) > 1 {
		args, err := shellquote.Split(parts[1])
		if err != nil {
			Debugf("executeCommand(): err: " + fmt.Sprint(err))
		} else {
			cmdArgs = args
		}
	}

	before := time.Now()
	out, err := exec.Command(cmd, cmdArgs...).CombinedOutput()
	duration := time.Since(before).Seconds()
	er := ExecResult{0, string(out)}
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		er.returnCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}
	mutex.Lock()
	syncGitTime += duration
	mutex.Unlock()
	Verbosef("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	if err != nil {
		if !allowFail {
			log.Print("executeCommand(): git command failed: " + command + " " + fmt.Sprint(err))
			log.Print("executeCommand(): Output: " + string(out))
			log.Println("If you are using GitLab be sure that you added your deploy key to your repository")
			os.Exit(1)
		} else {
			er.returnCode = 1
			er.output = fmt.Sprint(err)
		}
	}
	return er
}
