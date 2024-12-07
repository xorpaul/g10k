package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xorpaul/g10k/internal"
	"github.com/xorpaul/g10k/internal/config"
	"github.com/xorpaul/g10k/internal/fsutils"
	"github.com/xorpaul/g10k/internal/logging"
)

var (
	force                        bool
	usemove                      bool
	pfMode                       bool
	pfLocation                   string
	clonegit                     bool
	check4update                 bool
	checkSum                     bool
	moduleDirParam               string
	cacheDirParam                string
	branchParam                  string
	environmentParam             string
	tags                         bool
	outputNameParam              string
	moduleParam                  string
	configFile                   string
	GlobalConfig                 config.ConfigSettings
	mutex                        sync.Mutex
	empty                        struct{}
	syncGitCount                 int
	syncForgeCount               int
	needSyncGitCount             int
	needSyncForgeCount           int
	needSyncDirs                 []string
	needSyncEnvs                 map[string]struct{}
	syncGitTime                  float64
	syncForgeTime                float64
	ioGitTime                    float64
	ioForgeTime                  float64
	forgeJSONParseTime           float64
	metadataJSONParseTime        float64
	buildtime                    string
	buildversion                 string
	uniqueForgeModules           map[string]ForgeModule
	latestForgeModules           LatestForgeModules
	forgeModuleDeprecationNotice string
)

// LatestForgeModules contains a map of unique Forge modules
// that should be the latest versions of them
type LatestForgeModules struct {
	sync.RWMutex
	m map[string]string
}

// Puppetfile contains the key value pairs from the Puppetfile
type Puppetfile struct {
	forgeBaseURL      string
	forgeCacheTTL     time.Duration
	forgeModules      map[string]ForgeModule
	gitModules        map[string]GitModule
	privateKey        string
	source            string
	sourceBranch      string
	workDir           string
	gitDir            string
	gitURL            string
	moduleDirs        []string
	controlRepoBranch string
}

// ForgeModule contains information (Version, Name, Author, md5 checksum, file size of the tar.gz archive, Forge BaseURL if custom) about a Puppetlabs Forge module
type ForgeModule struct {
	version      string
	name         string
	author       string
	md5sum       string
	fileSize     int64
	baseURL      string
	cacheTTL     time.Duration
	sha256sum    string
	moduleDir    string
	sourceBranch string
}

// GitModule contains information about a Git Puppet module
type GitModule struct {
	privateKey        string
	git               string
	branch            string
	tag               string
	commit            string
	ref               string
	tree              string
	link              bool
	ignoreUnreachable bool
	fallback          []string
	installPath       string
	local             bool
	moduleDir         string
	useSSHAgent       bool
}

// ForgeResult is returned by queryForgeAPI and contains if and which version of the Puppetlabs Forge module needs to be downloaded
type ForgeResult struct {
	needToGet     bool
	versionNumber string
	md5sum        string
	fileSize      int64
}

// ExecResult contains the exit code and output of an external command (e.g. git)
type ExecResult struct {
	returnCode int
	output     string
}

// DeployResult contains information about the Puppet environment which was deployed by g10k and tries to emulate the .r10k-deploy.json
type DeployResult struct {
	Name               string    `json:"name"`
	Signature          string    `json:"signature"`
	StartedAt          time.Time `json:"started_at"`
	FinishedAt         time.Time `json:"finished_at"`
	DeploySuccess      bool      `json:"deploy_success"`
	PuppetfileChecksum string    `json:"puppetfile_checksum"`
	GitDir             string    `json:"git_dir"`
	GitURL             string    `json:"git_url"`
}

func init() {
	// initialize global maps
	needSyncEnvs = make(map[string]struct{})
	uniqueForgeModules = make(map[string]ForgeModule)
}

func main() {

	var (
		configFileFlag = flag.String("config", "", "which config file to use")
		versionFlag    = flag.Bool("version", false, "show build time and version number")
	)
	flag.StringVar(&branchParam, "branch", "", "which git branch of the Puppet environment to update. Just the branch name, e.g. master, qa, dev")
	flag.StringVar(&environmentParam, "environment", "", "which Puppet environment to update. Source name inside the config + '_' + branch name, e.g. foo_master, foo_qa, foo_dev")
	flag.BoolVar(&tags, "tags", false, "to pull tags as well as branches")
	flag.StringVar(&outputNameParam, "outputname", "", "overwrite the environment name if -branch is specified")
	flag.StringVar(&moduleParam, "module", "", "which module of the Puppet environment to update, e.g. stdlib")
	flag.StringVar(&moduleDirParam, "moduledir", "", "allows overriding of Puppetfile specific moduledir setting, the folder in which Puppet modules will be extracted")
	flag.StringVar(&cacheDirParam, "cachedir", "", "allows overriding of the g10k config file cachedir setting, the folder in which g10k will download git repositories and Forge modules")
	flag.IntVar(&config.Maxworker, "maxworker", 50, "how many Goroutines are allowed to run in parallel for Git and Forge module resolving")
	flag.IntVar(&config.MaxExtractworker, "maxextractworker", 20, "how many Goroutines are allowed to run in parallel for local Git and Forge module extracting processes (git clone, untar and gunzip)")
	flag.BoolVar(&pfMode, "puppetfile", false, "install all modules from Puppetfile in cwd")
	flag.StringVar(&pfLocation, "puppetfilelocation", "./Puppetfile", "which Puppetfile to use in -puppetfile mode")
	flag.BoolVar(&clonegit, "clonegit", false, "populate the Puppet environment with a git clone of each git Puppet module. Helpful when developing locally with -puppetfile")
	flag.BoolVar(&force, "force", false, "purge the Puppet environment directory and do a full sync")
	flag.BoolVar(&internal.DryRun, "dryrun", false, "do not modify anything, just print what would be changed")
	flag.BoolVar(&logging.Validate, "validate", false, "only validate given configuration and exit")
	flag.BoolVar(&usemove, "usemove", false, "do not use hardlinks to populate your Puppet environments with Puppetlabs Forge modules. Instead uses simple move commands and purges the Forge cache directory after each run! (Useful for g10k runs inside a Docker container)")
	flag.BoolVar(&check4update, "check4update", false, "only check if the is newer version of the Puppet module avaialable. Does implicitly set dryrun to true")
	flag.BoolVar(&checkSum, "checksum", false, "get the md5 check sum for each Puppetlabs Forge module and verify the integrity of the downloaded archive. Increases g10k run time!")
	flag.BoolVar(&logging.Debug, "debug", false, "log debug output, defaults to false")
	flag.BoolVar(&logging.Verbose, "verbose", false, "log verbose output, defaults to false")
	flag.BoolVar(&logging.Info, "info", false, "log info output, defaults to false")
	flag.BoolVar(&logging.Quiet, "quiet", false, "no output, defaults to false")
	flag.BoolVar(&config.UseCacheFallback, "usecachefallback", false, "if g10k should try to use its cache for sources and modules instead of failing")
	flag.BoolVar(&config.RetryGitCommands, "retrygitcommands", false, "if g10k should purge the local repository and retry a failed git command (clone or remote update) instead of failing")
	flag.BoolVar(&config.GitObjectSyntaxNotSupported, "gitobjectsyntaxnotsupported", false, "if your git version is too old to support reference syntax like master^{object} use this setting to revert to the older syntax")
	flag.Parse()

	configFile = *configFileFlag
	version := *versionFlag

	if version {
		fmt.Println("g10k ", buildversion, " Build time:", buildtime, "UTC")
		os.Exit(0)
	}

	if check4update {
		internal.DryRun = true
	}

	// check for git executable dependency
	if _, err := exec.LookPath("git"); err != nil {
		logging.Fatalf("Error: could not find 'git' executable in PATH")
	}

	target := ""
	before := time.Now()
	if len(configFile) > 0 {
		if usemove {
			logging.Fatalf("Error: -usemove parameter is only allowed in -puppetfile mode!")
		}
		if pfMode {
			logging.Fatalf("Error: -puppetfile parameter is not allowed with -config parameter!")
		}
		if (len(outputNameParam) > 0) && (len(branchParam) == 0) {
			logging.Fatalf("Error: -outputname specified without -branch!")
		}
		if config.UseCacheFallback {
			GlobalConfig.UseCacheFallback = true
		}
		logging.Debugf("Using as config file: " + configFile)
		GlobalConfig = config.ReadConfigFile(configFile)
		fsutils.CheckDirAndCreate(GlobalConfig.CacheDir, "cachedir configured value")
		target = configFile
		if len(branchParam) > 0 {
			resolvePuppetEnvironment(tags, outputNameParam)
			target += " with branch " + branchParam
		} else {
			branchParam = ""
			resolvePuppetEnvironment(tags, "")
		}
	} else {
		if pfMode {
			logging.Debugf("Trying to use as Puppetfile: " + pfLocation)
			sm := make(map[string]config.Source)
			sm["cmdlineparam"] = config.Source{Basedir: "./"}
			cachedir := "/tmp/g10k"
			if len(os.Getenv("g10k_cachedir")) > 0 {
				cachedir = os.Getenv("g10k_cachedir")
				cachedir = fsutils.CheckDirAndCreate(cachedir, "cachedir environment variable g10k_cachedir")
				logging.Debugf("Found environment variable g10k_cachedir set to: " + cachedir)
			} else if len(cacheDirParam) > 0 {
				logging.Debugf("Using -cachedir parameter set to : " + cacheDirParam)
				cachedir = fsutils.CheckDirAndCreate(cacheDirParam, "cachedir CLI param")
			} else {
				cachedir = fsutils.CheckDirAndCreate(cachedir, "cachedir default value")
			}
			forgeCachedir := fsutils.CheckDirAndCreate(filepath.Join(cachedir, "forge"), "default in pfMode")
			modulesCacheDir := fsutils.CheckDirAndCreate(filepath.Join(cachedir, "modules"), "default in pfMode")
			envsCacheDir := fsutils.CheckDirAndCreate(filepath.Join(cachedir, "environments"), "default in pfMode")
			GlobalConfig = config.ConfigSettings{
				CacheDir:                    cachedir,
				ForgeCacheDir:               forgeCachedir,
				ModulesCacheDir:             modulesCacheDir,
				EnvCacheDir:                 envsCacheDir,
				Sources:                     sm,
				ForgeBaseURL:                "https://forgeapi.puppet.com",
				Maxworker:                   config.Maxworker,
				UseCacheFallback:            config.UseCacheFallback,
				MaxExtractworker:            config.MaxExtractworker,
				RetryGitCommands:            config.RetryGitCommands,
				GitObjectSyntaxNotSupported: config.GitObjectSyntaxNotSupported,
			}
			// default purge_levels
			GlobalConfig.PurgeLevels = []string{"puppetfile"}
			if clonegit {
				GlobalConfig.CloneGitModules = true
			}
			target = pfLocation
			puppetfile := readPuppetfile(target, "", "cmdlineparam", "cmdlineparam", false, false)
			puppetfile.workDir = ""
			pfm := make(map[string]Puppetfile)
			pfm["cmdlineparam"] = puppetfile
			resolvePuppetfile(pfm)
		} else {
			logging.Fatalf("Error: you need to specify at least a config file or use the Puppetfile mode\nExample call: " + os.Args[0] + " -config test.yaml or " + os.Args[0] + " -puppetfile\n")
		}
	}

	if usemove {
		// we can not reuse the Forge cache at all when -usemove gets used, because we can not delete the -latest link for some reason
		defer fsutils.PurgeDir(GlobalConfig.ForgeCacheDir, "main() -puppetfile mode with -usemove parameter")
	}

	logging.Debugf("Forge response JSON parsing took " + strconv.FormatFloat(forgeJSONParseTime, 'f', 4, 64) + " seconds")
	logging.Debugf("Forge modules metadata.json parsing took " + strconv.FormatFloat(metadataJSONParseTime, 'f', 4, 64) + " seconds")

	if !check4update && !logging.Quiet {
		if len(forgeModuleDeprecationNotice) > 0 {
			logging.Warnf(strings.TrimSuffix(forgeModuleDeprecationNotice, "\n"))
		}
		fmt.Println("Synced", target, "with", syncGitCount, "git repositories and", syncForgeCount, "Forge modules in "+strconv.FormatFloat(time.Since(before).Seconds(), 'f', 1, 64)+"s with git ("+strconv.FormatFloat(syncGitTime, 'f', 1, 64)+"s sync, I/O", strconv.FormatFloat(ioGitTime, 'f', 1, 64)+"s) and Forge ("+strconv.FormatFloat(syncForgeTime, 'f', 1, 64)+"s query+download, I/O", strconv.FormatFloat(ioForgeTime, 'f', 1, 64)+"s) using", strconv.Itoa(GlobalConfig.Maxworker), "resolve and", strconv.Itoa(GlobalConfig.MaxExtractworker), "extract workers")
	}
	if internal.DryRun && (needSyncForgeCount > 0 || needSyncGitCount > 0) {
		os.Exit(1)
	}

	checkForAndExecutePostrunCommand()
}
