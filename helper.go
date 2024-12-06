package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/xorpaul/g10k/internal/logging"
	"golang.org/x/sys/unix"
)

var validationMessages []string

// fileExists checks if the given file exists and returns a bool
func fileExists(file string) bool {
	//logging.Debugf("checking for file existence " + file)
	if _, err := os.Lstat(file); os.IsNotExist(err) {
		return false
	}
	return true
}

// isDir checks if the given dir exists and returns a bool
func isDir(dir string) bool {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false
	}
	if fi.Mode().IsDir() {
		return true
	}
	return false
}

// normalizeDir removes from the given directory path multiple redundant slashes and removes a trailing slash
func normalizeDir(dir string) string {
	if strings.Count(dir, "//") > 0 {
		dir = normalizeDir(strings.Replace(dir, "//", "/", -1))
	}
	dir = strings.TrimSuffix(dir, "/")
	return dir
}

// checkDirAndCreate tests if the given directory exists and tries to create it
func checkDirAndCreate(dir string, name string) string {
	if !dryRun {
		if len(dir) != 0 {
			if !fileExists(dir) {
				//log.Printf("checkDirAndCreate(): trying to create dir '%s' as %s", dir, name){
				if err := os.MkdirAll(dir, 0777); err != nil {
					logging.Fatalf("checkDirAndCreate(): Error: failed to create directory: " + dir)
				}
			} else {
				if !isDir(dir) {
					logging.Fatalf("checkDirAndCreate(): Error: " + dir + " exists, but is not a directory! Exiting!")
				} else {
					if unix.Access(dir, unix.W_OK) != nil {
						logging.Fatalf("checkDirAndCreate(): Error: " + dir + " exists, but is not writable! Exiting!")
					}
				}
			}
		} else {
			// TODO make dir optional
			logging.Fatalf("checkDirAndCreate(): Error: dir setting '" + name + "' missing! Exiting!")
		}
	}
	dir = normalizeDir(dir)
	logging.Debugf("Using as " + name + ": " + dir)
	return dir
}

func createOrPurgeDir(dir string, callingFunction string) {
	if !dryRun {
		if !fileExists(dir) {
			logging.Debugf("Trying to create dir: " + dir + " called from " + callingFunction)
			os.MkdirAll(dir, 0777)
		} else {
			logging.Debugf("Trying to remove: " + dir + " called from " + callingFunction)
			if err := os.RemoveAll(dir); err != nil {
				log.Print("createOrPurgeDir(): error: removing dir failed", err)
			}
			logging.Debugf("Trying to create dir: " + dir + " called from " + callingFunction)
			os.MkdirAll(dir, 0777)
		}
	}
}

func purgeDir(dir string, callingFunction string) {
	if !fileExists(dir) {
		logging.Debugf("Unnecessary to remove dir: " + dir + " it does not exist. Called from " + callingFunction)
	} else {
		logging.Debugf("Trying to remove: " + dir + " called from " + callingFunction)
		if err := os.RemoveAll(dir); err != nil {
			log.Print("purgeDir(): os.RemoveAll() error: removing dir failed: ", err.Error())
			if err = syscall.Unlink(dir); err != nil {
				log.Print("purgeDir(): syscall.Unlink() error: removing link failed: ", err.Error())
			}
		}
	}
}

func executeCommand(command string, commandDir string, timeout int, allowFail bool, disableHTTPProxy bool) ExecResult {
	if len(commandDir) > 0 {
		logging.Debugf("Executing " + command + " in cwd " + commandDir)
	} else {
		logging.Debugf("Executing " + command)
	}
	parts := strings.SplitN(command, " ", 2)
	cmd := parts[0]
	cmdArgs := []string{}
	if len(parts) > 1 {
		args, err := shellquote.Split(parts[1])
		if err != nil {
			logging.Debugf("err: " + fmt.Sprint(err))
		} else {
			cmdArgs = args
		}
	}

	before := time.Now()
	execCommand := exec.Command(cmd, cmdArgs...)
	if len(commandDir) > 0 {
		execCommand.Dir = commandDir
	}
	if disableHTTPProxy {
		logging.Debugf("found matching NO_PROXY URL, trying to disable http_proxy and https_proxy env variables for " + command)
		// execCommand.Env = append(os.Environ(), "http_proxy=")
		// execCommand.Env = append(os.Environ(), "https_proxy=")
		os.Unsetenv("http_proxy")
		os.Unsetenv("https_proxy")
		os.Unsetenv("HTTP_PROXY")
		os.Unsetenv("HTTPS_PROXY")
	}
	execCommand.Env = os.Environ()
	out, err := execCommand.CombinedOutput()
	duration := time.Since(before).Seconds()
	er := ExecResult{0, string(out)}
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		er.returnCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}
	if (allowFail || config.UseCacheFallback) && err != nil {
		logging.Debugf("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	} else {
		logging.Verbosef("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	}
	if err != nil {
		er.returnCode = 1
		er.output = fmt.Sprint(err) + " " + fmt.Sprint(string(out))
	}
	return er
}

func timeTrack(start time.Time, name string) {
	duration := time.Since(start).Seconds()
	if name == "resolveForgeModules" {
		syncForgeTime = duration
	} else if name == "resolveGitRepositories" {
		syncGitTime = duration
	}
	logging.Debugf(name + "() took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
}

// checkForAndExecutePostrunCommand check if a `postrun` command was specified in the g10k config and executes it
func checkForAndExecutePostrunCommand() {
	if len(config.PostRunCommand) > 0 {
		postrunCommandString := strings.Join(config.PostRunCommand, " ")
		postrunCommandString = strings.Replace(postrunCommandString, "$modifieddirs", strings.Join(needSyncDirs, " "), -1)

		needSyncEnvText := ""
		for needSyncEnv := range needSyncEnvs {
			needSyncEnvText += needSyncEnv + " "
		}
		postrunCommandString = strings.Replace(postrunCommandString, "$modifiedenvs", needSyncEnvText, -1)
		postrunCommandString = strings.Replace(postrunCommandString, "$branchparam", branchParam, -1)

		er := executeCommand(postrunCommandString, "", config.Timeout, false, false)
		logging.Debugf("postrun command '" + postrunCommandString + "' terminated with exit code " + strconv.Itoa(er.returnCode))
	}
}

// getSha256sumFile return the SHA256 hash sum of the given file
func getSha256sumFile(file string) string {
	// https://golang.org/pkg/crypto/sha256/#New
	f, err := os.Open(file)
	if err != nil {
		logging.Fatalf("failed to open file " + file + " to calculate SHA256 sum. Error: " + err.Error())
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		logging.Fatalf("failed to calculate SHA256 sum of file " + file + " Error: " + err.Error())
	}

	return hex.EncodeToString(h.Sum(nil))
}

// moveFile uses io.Copy to create a copy of the given file https://stackoverflow.com/a/50741908/682847
func moveFile(sourcePath, destPath string, deleteSourceFileToggle bool) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("couldn't open source file: %s", err)
	}
	outputFile, err := os.Create(destPath)
	if err != nil {
		inputFile.Close()
		return fmt.Errorf("couldn't open dest file: %s", err)
	}
	defer outputFile.Close()
	_, err = io.Copy(outputFile, inputFile)
	inputFile.Close()
	if err != nil {
		return fmt.Errorf("writing to output file failed: %s", err)
	}
	if deleteSourceFileToggle {
		// The copy was successful, so now delete the original file
		err = os.Remove(sourcePath)
		if err != nil {
			return fmt.Errorf("failed removing original file: %s", err)
		}
	}
	return nil
}

func stringSliceContains(slice []string, element string) bool {
	for _, e := range slice {
		if e == element {
			return true
		}
	}
	return false
}

func writeStructJSONFile(file string, v interface{}) {
	content, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		logging.Warnf("Could not encode JSON file " + file + " " + err.Error())
	}

	err = ioutil.WriteFile(file, content, 0644)
	if err != nil {
		logging.Warnf("Could not write JSON file " + file + " " + err.Error())
	}

}

func readDeployResultFile(file string) DeployResult {
	// Open our jsonFile
	jsonFile, err := os.Open(file)
	// if we os.Open returns an error then handle it
	if err != nil {
		logging.Warnf("Could not open JSON file " + file + " " + err.Error())
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		logging.Warnf("Could not read JSON file " + file + " " + err.Error())
	}

	var dr DeployResult
	json.Unmarshal([]byte(byteValue), &dr)

	return dr

}

func stripComponent(component string, env string) string {
	if regexp.MustCompile(`^/.*/$`).MatchString(component) {
		return regexp.MustCompile(component[1:len(component)-1]).ReplaceAllString(env, "")
	}
	return strings.TrimPrefix(env, component)
}

func matchGitRemoteURLNoProxy(url string) bool {
	noProxy := os.Getenv("NO_PROXY")
	for _, np := range strings.Split(noProxy, ",") {
		if len(np) > 0 {
			if strings.Contains(url, np) {
				logging.Debugf("found NO_PROXY setting: " + np + " matching  " + url)
				return true
			}
		}
	}
	// do the same for lower case environment variable name
	noProxyL := os.Getenv("no_proxy")
	for _, np := range strings.Split(noProxyL, ",") {
		if len(np) > 0 {
			if strings.Contains(url, np) {
				logging.Debugf("found no_proxy setting: " + np + " matching  " + url)
				return true
			}
		}
	}
	return false
}
