package main

import (
	"path/filepath"
	"strings"

	"github.com/xorpaul/g10k/internal/logging"
)

func purgeUnmanagedContent(allBasedirs map[string]bool, allEnvironments map[string]bool) {
	if !stringSliceContains(config.PurgeLevels, "deployment") {
		if !stringSliceContains(config.PurgeLevels, "environment") {
			// nothing allowed to purge
			return
		}
	}
	for source, sa := range config.Sources {
		// fmt.Printf("source: %+v\n", sa)
		prefix := resolveSourcePrefix(source, sa)

		if len(environmentParam) > 0 {
			if !strings.HasPrefix(environmentParam, prefix) {
				logging.Debugf("Skipping purging unmanaged content for source '" + source + "', because -environment parameter is set to " + environmentParam)
				continue
			}
		}

		// Clean up unknown environment directories
		if len(branchParam) == 0 {
			for basedir := range allBasedirs {
				globPath := filepath.Join(basedir, prefix+"*")
				logging.Debugf("Glob'ing with path " + globPath)
				environments, _ := filepath.Glob(globPath)

				allowlistEnvironments := []string{}
				if len(config.DeploymentPurgeAllowList) > 0 {
					for _, wlpattern := range config.DeploymentPurgeAllowList {
						allowlistGlobPath := filepath.Join(basedir, wlpattern)
						logging.Debugf("deployment_purge_allowlist Glob'ing with path " + allowlistGlobPath)
						we, _ := filepath.Glob(allowlistGlobPath)
						allowlistEnvironments = append(allowlistEnvironments, we...)
					}
				}

				for _, env := range environments {
					envPath := strings.Split(env, "/")
					envName := envPath[len(envPath)-1]
					if len(environmentParam) > 0 {
						if envName != environmentParam {
							logging.Debugf("Skipping purging unmanaged content for Puppet environment '" + envName + "', because -environment parameter is set to " + environmentParam)
							continue
						}
					}
					if stringSliceContains(config.PurgeLevels, "deployment") {
						logging.Debugf("Checking if environment should exist: " + env)
						if allEnvironments[env] {
							logging.Debugf("Not purging environment " + env)
						} else if stringSliceContains(allowlistEnvironments, env) {
							logging.Debugf("Not purging environment " + env + " due to deployment_purge_allowlist match")
						} else {
							logging.Infof("Removing unmanaged environment " + env)
							if !dryRun {
								purgeDir(env, "purgeStaleContent()")
							}
						}
					}
				}
			}
		}
	}
}

func purgeControlRepoExceptModuledir(dir string, moduleDir string) {
	moduleDir = filepath.Join(dir, moduleDir)

	globPath := filepath.Join(dir, "*")
	logging.Debugf("Glob'ing with path " + globPath)
	folders, _ := filepath.Glob(globPath)
	for _, folder := range folders {
		if folder == moduleDir || strings.HasPrefix(folder, moduleDir) {
			continue
		} else {
			logging.Debugf("deleting " + folder)
			purgeDir(folder, "purgeControlRepoExceptModuledir")
		}

	}

}
