package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/xorpaul/g10k/internal/logging"
)

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
