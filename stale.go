package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/yargevad/filepathx"
)

func purgeUnmanagedContent(allBasedirs map[string]bool, allEnvironments map[string]bool) {
	if !stringSliceContains(config.PurgeLevels, "deployment") {
		if !stringSliceContains(config.PurgeLevels, "environment") {
			// nothing allowed to purge
			return
		}
	}

	for source, sa := range config.Sources {
		prefix := resolveSourcePrefix(source, sa)

		if len(environmentParam) > 0 {
			if !strings.HasPrefix(environmentParam, prefix) {
				Debugf("Skipping purging unmanaged content for source '" + source + "', because -environment parameter is set to " + environmentParam)
				continue
			}
		}

		// Clean up unknown environment directories
		if len(branchParam) == 0 {
			for basedir := range allBasedirs {
				globPath := filepath.Join(basedir, prefix+"*")
				Debugf("Glob'ing with path " + globPath)
				environments, _ := filepath.Glob(globPath)

				allowlistEnvironments := []string{}
				if len(config.DeploymentPurgeAllowList) > 0 {
					for _, wlpattern := range config.DeploymentPurgeAllowList {
						allowlistGlobPath := filepath.Join(basedir, wlpattern)
						Debugf("deployment_purge_allowlist Glob'ing with path " + allowlistGlobPath)
						we, _ := filepath.Glob(allowlistGlobPath)
						allowlistEnvironments = append(allowlistEnvironments, we...)
					}
				}

				for _, env := range environments {
					envPath := strings.Split(env, "/")
					envName := envPath[len(envPath)-1]
					if len(environmentParam) > 0 {
						if envName != environmentParam {
							Debugf("Skipping purging unmanaged content for Puppet environment '" + envName + "', because -environment parameter is set to " + environmentParam)
							continue
						}
					}
					if stringSliceContains(config.PurgeLevels, "environment") {
						if allEnvironments[envName] {
							checkForStaleContent(env)
						}
					}
					if stringSliceContains(config.PurgeLevels, "deployment") {
						Debugf("Checking if environment should exist: " + envName)
						if allEnvironments[envName] {
							Debugf("Not purging environment " + envName)
						} else if stringSliceContains(allowlistEnvironments, filepath.Join(basedir, envName)) {
							Debugf("Not purging environment " + envName + " due to deployment_purge_allowlist match")
						} else {
							Infof("Removing unmanaged environment " + envName)
							if !dryRun {
								purgeDir(filepath.Join(basedir, envName), "purgeStaleContent()")
							}
						}
					}
				}
			}
		} else {
			if stringSliceContains(config.PurgeLevels, "environment") {
				// check for purgeable content inside -branch folder
				checkForStaleContent(filepath.Join(sa.Basedir, prefix+branchParam))
			}
		}
	}
}

func checkForStaleContent(workDir string) {
	// add purge allowlist
	if len(config.PurgeAllowList) > 0 {
		for _, wlItem := range config.PurgeAllowList {
			Debugf("interpreting purge allowlist globs: " + strings.Join(config.PurgeAllowList, " "))
			globPath := filepath.Join(workDir, wlItem)
			Debugf("Glob'ing with purge allowlist glob " + globPath)
			wlPaths, _ := filepathx.Glob(globPath)
			Debugf("additional purge allowlist items: " + strings.Join(wlPaths, " "))
			desiredContent = append(desiredContent, wlPaths...)
		}
	}

	checkForStaleContent := func(path string, info os.FileInfo, err error) error {
		//Debugf("filepath.Walk'ing found path: " + path)
		stale := true
		if strings.HasSuffix(path, ".resource_types") && isDir(path) {
			stale = false
		} else {
			for _, desiredFile := range desiredContent {
				for _, unchangedModuleDir := range unchangedModuleDirs {
					if strings.HasPrefix(path, unchangedModuleDir) {
						stale = false
						break
					}
				}
				if path == desiredFile || path == workDir {
					stale = false
					break
				}
			}
		}

		if stale {
			Infof("Removing unmanaged path " + path)
			purgeDir(path, "checkForStaleContent()")
		}
		return nil
	}

	c := make(chan error)
	Debugf("filepath.Walk'ing directory " + workDir)
	go func() { c <- filepath.Walk(workDir, checkForStaleContent) }()
	<-c // Walk done
}
