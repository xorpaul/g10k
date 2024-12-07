package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/xorpaul/g10k/internal/fsutils"
	"github.com/xorpaul/g10k/internal/logging"
	"gopkg.in/yaml.v2"
)

var GlobalConfig ConfigSettings
var Maxworker int = 50
var MaxExtractworker int = 20
var UseCacheFallback bool
var RetryGitCommands bool
var GitObjectSyntaxNotSupported bool

// ConfigSettings contains the key value pairs from the g10k config file
type ConfigSettings struct {
	CacheDir                    string `yaml:"cachedir"`
	ForgeCacheDir               string
	ModulesCacheDir             string
	EnvCacheDir                 string
	Git                         Git
	Sources                     map[string]Source
	Timeout                     int             `yaml:"timeout" default:"300"`
	IgnoreUnreachableModules    bool            `yaml:"ignore_unreachable_modules"`
	Maxworker                   int             `yaml:"maxworker"`
	MaxExtractworker            int             `yaml:"maxextractworker"`
	UseCacheFallback            bool            `yaml:"use_cache_fallback"`
	RetryGitCommands            bool            `yaml:"retry_git_commands"`
	GitObjectSyntaxNotSupported bool            `yaml:"git_object_syntax_not_supported"`
	PostRunCommand              []string        `yaml:"postrun"`
	Deploy                      *DeploySettings `yaml:"deploy"`
	PurgeLevels                 []string        `yaml:"purge_levels"`
	PurgeAllowList              []string        `yaml:"purge_allowlist"`
	DeploymentPurgeAllowList    []string        `yaml:"deployment_purge_allowlist"`
	WriteLock                   string          `yaml:"write_lock"`
	GenerateTypes               bool            `yaml:"generate_types"`
	PuppetPath                  string          `yaml:"puppet_path"`
	PurgeSkiplist               []string        `yaml:"purge_skiplist"`
	CloneGitModules             bool            `yaml:"clone_git_modules"`
	ForgeBaseURL                string          `yaml:"forge_base_url"`
	ForgeCacheTTLString         string          `yaml:"forge_cache_ttl"`
	ForgeCacheTTL               time.Duration
}

// DeploySettings is a struct for settings for controlling how g10k deploys behave.
// Trying to emulate r10k https://github.com/puppetlabs/r10k/blob/master/doc/dynamic-environments/configuration.mkd#deploy
type DeploySettings struct {
	PurgeLevels              []string `yaml:"purge_levels"`
	PurgeAllowList           []string `yaml:"purge_allowlist"`
	DeploymentPurgeAllowList []string `yaml:"deployment_purge_allowlist"`
	WriteLock                string   `yaml:"write_lock"`
	GenerateTypes            bool     `yaml:"generate_types"`
	PuppetPath               string   `yaml:"puppet_path"`
	PurgeSkiplist            []string `yaml:"purge_skiplist"`
}

// Forge is a simple struct that contains the base URL of
// the Forge that g10k should use. Defaults to: https://forgeapi.puppet.com
type Forge struct {
	Baseurl string `yaml:"baseurl"`
}

// Git is a simple struct that contains the optional SSH private key to
// use for authentication
type Git struct {
	PrivateKey string `yaml:"private_key"`
}

// Source contains basic information about a Puppet environment repository
type Source struct {
	Remote                      string
	Basedir                     string
	Prefix                      string
	PrivateKey                  string `yaml:"private_key"`
	ForceForgeVersions          bool   `yaml:"force_forge_versions"`
	WarnMissingBranch           bool   `yaml:"warn_if_branch_is_missing"`
	ErrorMissingBranch          bool   `yaml:"error_if_branch_is_missing"`
	ExitIfUnreachable           bool   `yaml:"exit_if_unreachable"`
	AutoCorrectEnvironmentNames string `yaml:"invalid_branches"`
	FilterCommand               string `yaml:"filter_command"`
	FilterRegex                 string `yaml:"filter_regex"`
	StripComponent              string `yaml:"strip_component"`
}

// config.ReadConfigFile creates the ConfigSettings struct from the g10k config file
func ReadConfigFile(configFile string) ConfigSettings {
	logging.Debugf("Trying to read g10k config file: " + configFile)
	data, err := os.ReadFile(configFile)
	if err != nil {
		logging.Fatalf("config.ReadConfigFile(): There was an error parsing the config file " + configFile + ": " + err.Error())
	}

	var config ConfigSettings
	err = yaml.Unmarshal(stripRubySymbols(data), &config)
	if err != nil {
		logging.Fatalf("YAML unmarshal error: " + err.Error())
	}

	if len(os.Getenv("g10k_cachedir")) > 0 {
		cachedir := os.Getenv("g10k_cachedir")
		logging.Debugf("Found environment variable g10k_cachedir set to: " + cachedir)
		config.CacheDir = fsutils.CheckDirAndCreate(cachedir, "cachedir environment variable g10k_cachedir")
	} else {
		config.CacheDir = fsutils.CheckDirAndCreate(config.CacheDir, "cachedir from g10k config "+configFile)
	}

	config.CacheDir = fsutils.CheckDirAndCreate(config.CacheDir, "cachedir")
	config.ForgeCacheDir = fsutils.CheckDirAndCreate(filepath.Join(config.CacheDir, "forge"), "cachedir/forge")
	config.ModulesCacheDir = fsutils.CheckDirAndCreate(filepath.Join(config.CacheDir, "modules"), "cachedir/modules")
	config.EnvCacheDir = fsutils.CheckDirAndCreate(filepath.Join(config.CacheDir, "environments"), "cachedir/environments")

	if len(config.ForgeBaseURL) == 0 {
		config.ForgeBaseURL = "https://forgeapi.puppet.com"
	}
	if config.Timeout == 0 {
		config.Timeout = 5
	}

	if UseCacheFallback {
		config.UseCacheFallback = true
	}

	if RetryGitCommands {
		config.RetryGitCommands = true
	}

	if GitObjectSyntaxNotSupported {
		config.GitObjectSyntaxNotSupported = true
	}

	config.Maxworker = Maxworker
	config.MaxExtractworker = MaxExtractworker

	if len(config.ForgeCacheTTLString) != 0 {
		ttl, err := time.ParseDuration(config.ForgeCacheTTLString)
		if err != nil {
			logging.Fatalf("Error: Can not convert value " + config.ForgeCacheTTLString + " of config setting forge_cache_ttl to a golang Duration. Valid time units are 300ms, 1.5h or 2h45m. In " + configFile)
		}
		config.ForgeCacheTTL = ttl
	}

	// check for non-empty config.Deploy which takes precedence over the non-deploy scoped settings
	// See https://github.com/puppetlabs/r10k/blob/master/doc/dynamic-environments/configuration.mkd#deploy

	if config.Deploy != nil {
		logging.Debugf("detected deploy configuration hash, which takes precedence over the non-deploy scoped settings")
		config.PurgeLevels = config.Deploy.PurgeLevels
		config.PurgeAllowList = config.Deploy.PurgeAllowList
		config.DeploymentPurgeAllowList = config.Deploy.DeploymentPurgeAllowList
		config.WriteLock = config.Deploy.WriteLock
		config.GenerateTypes = config.Deploy.GenerateTypes
		config.PuppetPath = config.Deploy.PuppetPath
		config.PurgeSkiplist = config.Deploy.PurgeSkiplist
		config.Deploy = nil
	}

	if len(config.PurgeLevels) == 0 {
		config.PurgeLevels = []string{"deployment", "puppetfile"}
	}

	for source, sa := range config.Sources {
		sa.Basedir = fsutils.NormalizeDir(sa.Basedir)

		// set default to "correct_and_warn" like r10k
		// https://github.com/puppetlabs/r10k/blob/master/doc/dynamic-environments/git-environments.mkd#invalid_branches
		if len(sa.AutoCorrectEnvironmentNames) == 0 {
			sa.AutoCorrectEnvironmentNames = "correct_and_warn"
		}
		config.Sources[source] = sa
	}

	if logging.Validate {
		logging.Validatef()
	}

	// fmt.Printf("%+v\n", config)
	return config
}

func stripRubySymbols(data []byte) []byte {
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
	return []byte(rubySymbolsRemoved)
}
