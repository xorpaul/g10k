package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

var (
	debug                  bool
	verbose                bool
	info                   bool
	quiet                  bool
	force                  bool
	usemove                bool
	usecacheFallback       bool
	retryGitCommands       bool
	pfMode                 bool
	pfLocation             string
	dryRun                 bool
	check4update           bool
	checkSum               bool
	moduleDirParam         string
	cacheDirParam          string
	branchParam            string
	moduleParam            string
	configFile             string
	config                 ConfigSettings
	mutex                  sync.Mutex
	empty                  struct{}
	syncGitCount           int
	syncForgeCount         int
	needSyncGitCount       int
	needSyncForgeCount     int
	syncGitTime            float64
	syncForgeTime          float64
	ioGitTime              float64
	ioForgeTime            float64
	forgeJsonParseTime     float64
	metadataJsonParseTime  float64
	gmetadataJsonParseTime float64
	buildtime              string
	uniqueForgeModules     map[string]ForgeModule
	latestForgeModules     LatestForgeModules
	maxworker              int
	maxExtractworker       int
)

type LatestForgeModules struct {
	sync.RWMutex
	m map[string]string
}

// ConfigSettings contains the key value pairs from the g10k config file
type ConfigSettings struct {
	CacheDir                 string `yaml:"cachedir"`
	ForgeCacheDir            string
	ModulesCacheDir          string
	EnvCacheDir              string
	Git                      Git
	Forge                    Forge
	Sources                  map[string]Source
	Timeout                  int  `yaml:"timeout"`
	IgnoreUnreachableModules bool `yaml:"ignore_unreachable_modules"`
	Maxworker                int  `yaml:"maxworker"`
	MaxExtractworker         int  `yaml:"maxextractworker"`
	UseCacheFallback         bool `yaml:"use_cache_fallback"`
	RetryGitCommands         bool `yaml:"retry_git_commands"`
}

type Forge struct {
	Baseurl string `yaml:"baseurl"`
}

type Git struct {
	privateKey string `yaml:"private_key"`
	username   string
}

// Source contains basic information about a Puppet environment repository
type Source struct {
	Remote             string
	Basedir            string
	Prefix             string
	PrivateKey         string `yaml:"private_key"`
	ForceForgeVersions bool   `yaml:"force_forge_versions"`
	WarnMissingBranch  bool   `yaml:"warn_if_branch_is_missing"`
	ExitIfUnreachable  bool   `yaml:"exit_if_unreachable"`
}

// Puppetfile contains the key value pairs from the Puppetfile
type Puppetfile struct {
	moduleDir     string
	forgeBaseURL  string
	forgeCacheTtl time.Duration
	forgeModules  map[string]ForgeModule
	gitModules    map[string]GitModule
	privateKey    string
	source        string
	workDir       string
	localModules  map[string]struct{}
}

// ForgeModule contains information (Version, Name, Author, md5 checksum, file size of the tar.gz archive, Forge BaseURL if custom) about a Puppetlabs Forge module
type ForgeModule struct {
	version   string
	name      string
	author    string
	md5sum    string
	fileSize  int64
	baseUrl   string
	cacheTtl  time.Duration
	sha256sum string
}

// GitModule contains information about a Git Puppet module
type GitModule struct {
	privateKey        string
	git               string
	branch            string
	tag               string
	commit            string
	ref               string
	link              bool
	ignoreUnreachable bool
	fallback          []string
	installPath       string
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

func main() {

	var (
		configFileFlag = flag.String("config", "", "which config file to use")
		versionFlag    = flag.Bool("version", false, "show build time and version number")
	)
	flag.StringVar(&branchParam, "branch", "", "which git branch of the Puppet environment to update, e.g. core_foobar")
	flag.StringVar(&moduleParam, "module", "", "which module of the Puppet environment to update, e.g. stdlib")
	flag.StringVar(&moduleDirParam, "moduledir", "", "allows overriding of Puppetfile specific moduledir setting, the folder in which Puppet modules will be extracted")
	flag.StringVar(&cacheDirParam, "cachedir", "", "allows overriding of the g10k config file cachedir setting, the folder in which g10k will download git repositories and Forge modules")
	flag.IntVar(&maxworker, "maxworker", 50, "how many Goroutines are allowed to run in parallel for Git and Forge module resolving")
	flag.IntVar(&maxExtractworker, "maxextractworker", 20, "how many Goroutines are allowed to run in parallel for local Git and Forge module extracting processes (git clone, untar and gunzip)")
	flag.BoolVar(&pfMode, "puppetfile", false, "install all modules from Puppetfile in cwd")
	flag.StringVar(&pfLocation, "puppetfilelocation", "./Puppetfile", "which Puppetfile to use in -puppetfile mode")
	flag.BoolVar(&force, "force", false, "purge the Puppet environment directory and do a full sync")
	flag.BoolVar(&dryRun, "dryrun", false, "do not modify anything, just print what would be changed")
	flag.BoolVar(&usemove, "usemove", false, "do not use hardlinks to populate your Puppet environments with Puppetlabs Forge modules. Instead uses simple move commands and purges the Forge cache directory after each run! (Useful for g10k runs inside a Docker container)")
	flag.BoolVar(&check4update, "check4update", false, "only check if the is newer version of the Puppet module avaialable. Does implicitly set dryrun to true")
	flag.BoolVar(&checkSum, "checksum", false, "get the md5 check sum for each Puppetlabs Forge module and verify the integrity of the downloaded archive. Increases g10k run time!")
	flag.BoolVar(&debug, "debug", false, "log debug output, defaults to false")
	flag.BoolVar(&verbose, "verbose", false, "log verbose output, defaults to false")
	flag.BoolVar(&info, "info", false, "log info output, defaults to false")
	flag.BoolVar(&quiet, "quiet", false, "no output, defaults to false")
	flag.BoolVar(&usecacheFallback, "usecachefallback", false, "if g10k should try to use its cache for sources and modules instead of failing")
	flag.BoolVar(&retryGitCommands, "retrygitcommands", false, "if g10k should purge the local repository and retry a failed git command (clone or remote update) instead of failing")
	flag.Parse()

	configFile = *configFileFlag
	version := *versionFlag

	if version {
		fmt.Println("g10k version 0.4.2 Build time:", buildtime, "UTC")
		os.Exit(0)
	}

	if check4update {
		dryRun = true
	}

	// check for git executable dependency
	if _, err := exec.LookPath("git"); err != nil {
		Fatalf("Error: could not find 'git' executable in PATH")
	}

	target := ""
	before := time.Now()
	if len(configFile) > 0 {
		if usemove {
			Fatalf("Error: -usemove parameter is only allowed in -puppetfile mode!")
		}
		if pfMode {
			Fatalf("Error: -puppetfile parameter is not allowed with -config parameter!")
		}
		if usecacheFallback {
			config.UseCacheFallback = true
		}
		Debugf("Using as config file: " + configFile)
		config = readConfigfile(configFile)
		target = configFile
		if len(branchParam) > 0 {
			resolvePuppetEnvironment(branchParam)
			target += " with branch " + branchParam
		} else {
			resolvePuppetEnvironment("")
		}
	} else {
		if pfMode {
			Debugf("Trying to use as Puppetfile: " + pfLocation)
			sm := make(map[string]Source)
			sm["cmdlineparam"] = Source{Basedir: "."}
			cachedir := "/tmp/g10k"
			if len(os.Getenv("g10k_cachedir")) > 0 {
				cachedir = os.Getenv("g10k_cachedir")
				cachedir = checkDirAndCreate(cachedir, "cachedir environment variable g10k_cachedir")
				Debugf("Found environment variable g10k_cachedir set to: " + cachedir)
			} else if len(cacheDirParam) > 0 {
				Debugf("Using -cachedir parameter set to : " + cacheDirParam)
				cachedir = checkDirAndCreate(cacheDirParam, "cachedir CLI param")
			} else {
				cachedir = checkDirAndCreate(cachedir, "cachedir default value")
			}
			//config = ConfigSettings{CacheDir: cachedir, ForgeCacheDir: cachedir, ModulesCacheDir: cachedir, EnvCacheDir: cachedir, Forge:{Baseurl: "https://forgeapi.puppetlabs.com"}, Sources: sm}
			forgeDefaultSettings := Forge{Baseurl: "https://forgeapi.puppetlabs.com"}
			config = ConfigSettings{CacheDir: cachedir, ForgeCacheDir: cachedir, ModulesCacheDir: cachedir, EnvCacheDir: cachedir, Sources: sm, Forge: forgeDefaultSettings, Maxworker: maxworker, UseCacheFallback: usecacheFallback, MaxExtractworker: maxExtractworker, RetryGitCommands: retryGitCommands}
			target = pfLocation
			puppetfile := readPuppetfile(target, "", "cmdlineparam", false)
			puppetfile.workDir = "."
			pfm := make(map[string]Puppetfile)
			pfm["cmdlineparam"] = puppetfile
			resolvePuppetfile(pfm)
		} else {
			Fatalf("Error: you need to specify at least a config file or use the Puppetfile mode\nExample call: " + os.Args[0] + " -config test.yaml or " + os.Args[0] + " -puppetfile\n")
		}
	}

	if usemove {
		// we can not reuse the Forge cache at all when -usemove gets used, because we can not delete the -latest link for some reason
		defer purgeDir(config.ForgeCacheDir, "main() -puppetfile mode with -usemove parameter")
	}

	// DEBUG
	//pf := make(map[string]Puppetfile)
	//pf["core_fullmanaged"] = readPuppetfile("/tmp/core/core_fullmanaged/", "/home/andpaul/dev/go/src/github.com/xorpaul/g10k/portal_envs")
	//pf["itodsi_corosync"] = readPuppetfile("/tmp/itodsi/itodsi_corosync/", "/home/andpaul/dev/go/src/github.com/xorpaul/g10k/portal_envs")
	//resolvePuppetfile(pf)
	//resolveGitRepositories(config)
	//resolveForgeModules(configSettings.forge)
	//doModuleInstallOrNothing("camptocamp-postfix-1.2.2", "/tmp/g10k/camptocamp-postfix-1.2.2")
	//doModuleInstallOrNothing("saz-resolv_conf-latest")
	//readModuleMetadata("/tmp/g10k/forge/camptocamp-postfix-1.2.2/metadata.json")

	Debugf("Forge response JSON parsing took " + strconv.FormatFloat(forgeJsonParseTime, 'f', 4, 64) + " seconds")
	Debugf("Forge modules metadata.json parsing took " + strconv.FormatFloat(metadataJsonParseTime, 'f', 4, 64) + " seconds")

	if !check4update && !quiet {
		fmt.Println("Synced", target, "with", syncGitCount, "git repositories and", syncForgeCount, "Forge modules in "+strconv.FormatFloat(time.Since(before).Seconds(), 'f', 1, 64)+"s with git ("+strconv.FormatFloat(syncGitTime, 'f', 1, 64)+"s sync, I/O", strconv.FormatFloat(ioGitTime, 'f', 1, 64)+"s) and Forge ("+strconv.FormatFloat(syncForgeTime, 'f', 1, 64)+"s query+download, I/O", strconv.FormatFloat(ioForgeTime, 'f', 1, 64)+"s) using", strconv.Itoa(config.Maxworker), "resolv and", strconv.Itoa(config.MaxExtractworker), "extract workers")
	}
	if dryRun && (needSyncForgeCount > 0 || needSyncGitCount > 0) {
		os.Exit(1)
	}
}
