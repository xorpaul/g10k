package main

import (
	"os"
	"path/filepath"
	"strings"
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
			for basedir, _ := range allBasedirs {
				globPath := filepath.Join(basedir, prefix+"*")
				Debugf("Glob'ing with path " + globPath)
				environments, _ := filepath.Glob(globPath)

				whitelistEnvironments := []string{}
				if len(config.DeploymentPurgeWhitelist) > 0 {
					for _, wlpattern := range config.DeploymentPurgeWhitelist {
						whitelistGlobPath := filepath.Join(basedir, wlpattern)
						Debugf("deployment_purge_whitelist Glob'ing with path " + whitelistGlobPath)
						we, _ := filepath.Glob(whitelistGlobPath)
						whitelistEnvironments = append(whitelistEnvironments, we...)
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
						} else if stringSliceContains(whitelistEnvironments, filepath.Join(basedir, envName)) {
							Debugf("Not purging environment " + envName + " due to deployment_purge_whitelist match")
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
	// add purge whitelist
	if len(config.PurgeWhitelist) > 0 {
		Debugf("additional purge whitelist items: " + strings.Join(config.PurgeWhitelist, " "))
		for _, wlItem := range config.PurgeWhitelist {
			desiredContent = append(desiredContent, filepath.Join(workDir, wlItem))
		}
	}

	checkForStaleContent := func(path string, info os.FileInfo, err error) error {
		//Debugf("filepath.Walk'ing found path: " + path)
		stale := true
		for _, desiredFile := range desiredContent {
			//Debugf("comparing found path: " + path + " with managed path: " + desiredFile)
			//if strings.HasPrefix(path, desiredFile) || path == workDir {
			if path == desiredFile || path == workDir {
				stale = false
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
