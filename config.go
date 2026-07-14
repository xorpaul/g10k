package main

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

var (
	reModuledir = regexp.MustCompile(`^\s*(?:moduledir)\s+['\"]?([^'\"]+)['\"]?`)
)

// readConfigfile creates the ConfigSettings struct from the g10k config file
func readConfigfile(configFile string) ConfigSettings {
	Debugf("Trying to read g10k config file: " + configFile)
	data, err := os.ReadFile(configFile)
	if err != nil {
		Fatalf("readConfigfile(): There was an error parsing the config file " + configFile + ": " + err.Error())
	}

	rubySymbolsRemoved := ""
	for _, line := range strings.Split(string(data), "\n") {
		reWhitespaceColon := regexp.MustCompile(`^(\s*):`)
		m := reWhitespaceColon.FindStringSubmatch(line)
		if len(m) > 0 {
			rubySymbolsRemoved += reWhitespaceColon.ReplaceAllString(line, m[1]) + "\n"
		} else {
			rubySymbolsRemoved += line + "\n"
		}
	}
	var config ConfigSettings
	err = yaml.Unmarshal([]byte(rubySymbolsRemoved), &config)
	if err != nil {
		Fatalf("YAML unmarshal error: " + err.Error())
	}

	if len(os.Getenv("g10k_cachedir")) > 0 {
		cachedir := os.Getenv("g10k_cachedir")
		Debugf("Found environment variable g10k_cachedir set to: " + cachedir)
		config.CacheDir = checkDirAndCreate(cachedir, "cachedir environment variable g10k_cachedir")
	} else {
		config.CacheDir = checkDirAndCreate(config.CacheDir, "cachedir from g10k config "+configFile)
	}

	config.CacheDir = checkDirAndCreate(config.CacheDir, "cachedir")
	config.ForgeCacheDir = checkDirAndCreate(filepath.Join(config.CacheDir, "forge"), "cachedir/forge")
	config.ModulesCacheDir = checkDirAndCreate(filepath.Join(config.CacheDir, "modules"), "cachedir/modules")
	config.EnvCacheDir = checkDirAndCreate(filepath.Join(config.CacheDir, "environments"), "cachedir/environments")

	if len(config.ForgeBaseURL) == 0 {
		config.ForgeBaseURL = "https://forgeapi.puppet.com"
	}

	// fmt.Println("Forge Baseurl: ", config.ForgeBaseURL)

	// set default timeout to 5 seconds if no timeout setting found
	if config.Timeout == 0 {
		config.Timeout = 5
	}

	if usecacheFallback {
		config.UseCacheFallback = true
	}

	if retryGitCommands {
		config.RetryGitCommands = true
	}

	if gitObjectSyntaxNotSupported {
		config.GitObjectSyntaxNotSupported = true
	}

	// set default max Go routines for Forge and Git module resolution if none is given
	if config.Maxworker <= 0 {
		config.Maxworker = maxworker
	}
	if maxworker != 50 {
		config.Maxworker = maxworker
	}

	if maxworker == 0 && config.Maxworker == 0 {
		config.Maxworker = 50
	}

	// set default max Go routines for Forge and Git module extracting
	if config.MaxExtractworker <= 0 {
		config.MaxExtractworker = maxExtractworker
	}
	if maxExtractworker != 20 {
		config.MaxExtractworker = maxExtractworker
	}

	if maxExtractworker == 0 && config.MaxExtractworker == 0 {
		config.MaxExtractworker = 20
	}

	if len(config.ForgeCacheTTLString) != 0 {
		ttl, err := time.ParseDuration(config.ForgeCacheTTLString)
		if err != nil {
			Fatalf("Error: Can not convert value " + config.ForgeCacheTTLString + " of config setting forge_cache_ttl to a golang Duration. Valid time units are 300ms, 1.5h or 2h45m. In " + configFile)
		}
		config.ForgeCacheTTL = ttl
	}

	// check for non-empty config.Deploy which takes precedence over the non-deploy scoped settings
	// See https://github.com/puppetlabs/r10k/blob/master/doc/dynamic-environments/configuration.mkd#deploy
	emptyDeploy := DeploySettings{}
	if !reflect.DeepEqual(config.Deploy, emptyDeploy) {
		Debugf("detected deploy configuration hash, which takes precedence over the non-deploy scoped settings")
		config.PurgeLevels = config.Deploy.PurgeLevels
		config.PurgeAllowList = config.Deploy.PurgeAllowList
		config.DeploymentPurgeAllowList = config.Deploy.DeploymentPurgeAllowList
		config.WriteLock = config.Deploy.WriteLock
		config.GenerateTypes = config.Deploy.GenerateTypes
		config.PuppetPath = config.Deploy.PuppetPath
		config.PurgeSkiplist = config.Deploy.PurgeSkiplist
		config.Deploy = emptyDeploy
	}

	if len(config.PurgeLevels) == 0 {
		config.PurgeLevels = []string{"deployment", "puppetfile"}
	}

	for source, sa := range config.Sources {
		sa.Basedir = normalizeDir(sa.Basedir)

		// set default to "correct_and_warn" like r10k
		// https://github.com/puppetlabs/r10k/blob/master/doc/dynamic-environments/git-environments.mkd#invalid_branches
		if len(sa.AutoCorrectEnvironmentNames) == 0 {
			sa.AutoCorrectEnvironmentNames = "correct_and_warn"
		}
		config.Sources[source] = sa
	}

	if validate {
		Validatef()
	}

	// fmt.Printf("%+v\n", config)
	return config
}

// preparePuppetfile remove whitespace and comment lines from the given Puppetfile and merges Puppetfile resources that are identified with having a , at the end
func preparePuppetfile(pf string) string {
	file, err := os.Open(pf)
	if err != nil {
		Fatalf("preparePuppetfile(): Error while opening Puppetfile " + pf + " Error: " + err.Error())
	}
	defer func() {
		_ = file.Close()
	}()

	reComma := regexp.MustCompile(`,\s*$`)
	reComment := regexp.MustCompile(`^\s*#`)
	reEmpty := regexp.MustCompile("^$")

	pfString := ""
	scanner := bufio.NewScanner(file)
	Debugf("scanning file: " + pf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !reComment.MatchString(line) && !reEmpty.MatchString(line) {
			if strings.Contains(line, "#") {
				Debugf("found inline comment in " + pf + "line: " + line)
				line = strings.Split(line, "#")[0]
			}
			if reComma.MatchString(line) {
				pfString += line
				Debugf("adding line:" + line)
			} else {
				pfString += line + "\n"
				Debugf("adding line:" + line)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		Fatalf("preparePuppetfile(): Error while scanning Puppetfile " + pf + " Error: " + err.Error())
	}

	return pfString
}

// readPuppetfile creates the Puppetfile struct from the Puppetfile using a custom goyacc parser
func readPuppetfile(pf string, sshKey string, source string, branch string, forceForgeVersions bool) Puppetfile {
	var puppetFile Puppetfile
	puppetFile.privateKey = sshKey
	puppetFile.source = source
	puppetFile.forgeModules = map[string]ForgeModule{}
	puppetFile.gitModules = map[string]GitModule{}

	Debugf("Trying to parse: " + pf)
	parsedPuppetfile := ParsePuppetfile(pf, sshKey, source, branch, forceForgeVersions)
	puppetFile = parsedPuppetfile

	if validate {
		Validatef()
	}

	return puppetFile
}
