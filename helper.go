package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/kballard/go-shellquote"
)

// Debugf is a helper function for debug logging if global variable debug is set to true
func Debugf(s string) {
	if debug != false {
		pc, _, _, _ := runtime.Caller(1)
		callingFunctionName := strings.Split(runtime.FuncForPC(pc).Name(), ".")[len(strings.Split(runtime.FuncForPC(pc).Name(), "."))-1]
		if strings.HasPrefix(callingFunctionName, "func") {
			// check for anonymous function names
			log.Print("DEBUG " + fmt.Sprint(s))
		} else {
			log.Print("DEBUG " + callingFunctionName + "(): " + fmt.Sprint(s))
		}
	}
}

// Verbosef is a helper function for verbose logging if global variable verbose is set to true
func Verbosef(s string) {
	if debug != false || verbose != false {
		log.Print(fmt.Sprint(s))
	}
}

// Infof is a helper function for info logging if global variable info is set to true
func Infof(s string) {
	if debug != false || verbose != false || info != false {
		color.Green(s)
	}
}

// Warnf is a helper function for warning logging
func Warnf(s string) {
	color.Set(color.FgYellow)
	fmt.Println(s)
	color.Unset()
}

// Fatalf is a helper function for fatal logging
func Fatalf(s string) {
	color.Set(color.FgRed)
	log.Fatal(s)
	color.Unset()
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
			if !fileExists(dir) {
				//log.Printf("checkDirAndCreate(): trying to create dir '%s' as %s", dir, name){
				if err := os.MkdirAll(dir, 0777); err != nil {
					Fatalf("checkDirAndCreate(): Error: failed to create directory: " + dir)
				}
			}
		} else {
			// TODO make dir optional
			Fatalf("checkDirAndCreate(): Error: dir setting '" + name + "' missing! Exiting!")
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
		if !fileExists(dir) {
			Debugf("Trying to create dir: " + dir + " called from " + callingFunction)
			os.Mkdir(dir, 0777)
		} else {
			Debugf("Trying to remove: " + dir + " called from " + callingFunction)
			if err := os.RemoveAll(dir); err != nil {
				log.Print("createOrPurgeDir(): error: removing dir failed", err)
			}
			Debugf("Trying to create dir: " + dir + " called from " + callingFunction)
			os.Mkdir(dir, 0777)
		}
	}
}

func purgeDir(dir string, callingFunction string) {
	if !fileExists(dir) {
		Debugf("Unnecessary to remove dir: " + dir + " it does not exist. Called from " + callingFunction)
	} else {
		Debugf("Trying to remove: " + dir + " called from " + callingFunction)
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
			Debugf("err: " + fmt.Sprint(err))
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
	if allowFail && err != nil {
		Debugf("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	} else {
		Verbosef("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	}
	if err != nil {
		if !allowFail {
			Fatalf("executeCommand(): git command failed: " + command + " " + err.Error() + "\nOutput: " + string(out) +
				"\nIf you are using GitLab be sure that you added your deploy key to your repository")
		} else {
			er.returnCode = 1
			er.output = fmt.Sprint(err)
		}
	}
	return er
}

// funcName return the function name as a string
func funcName() string {
	pc, _, _, _ := runtime.Caller(1)
	completeFuncname := runtime.FuncForPC(pc).Name()
	return strings.Split(completeFuncname, ".")[len(strings.Split(completeFuncname, "."))-1]
}

func timeTrack(start time.Time, name string) {
	duration := time.Since(start).Seconds()
	if name == "resolveForgeModules" {
		syncForgeTime = duration
	} else if name == "resolveGitRepositories" {
		syncGitTime = duration
	}
	Debugf(name + "() took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
}
