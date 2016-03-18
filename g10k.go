package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	debug              bool
	verbose            bool
	info               bool
	force              bool
	usemove            bool
	pfMode             bool
	dryRun             bool
	config             ConfigSettings
	wg                 sync.WaitGroup
	mutex              sync.Mutex
	empty              struct{}
	syncGitCount       int
	syncForgeCount     int
	needSyncGitCount   int
	needSyncForgeCount int
	syncGitTime        float64
	syncForgeTime      float64
	cpGitTime          float64
	cpForgeTime        float64
	buildtime          string
	uniqueForgeModules map[string]struct{}
	latestForgeModules map[string]string
)

// ConfigSettings contains the key value pairs from the g10k config file
type ConfigSettings struct {
	CacheDir        string `yaml:"cachedir"`
	ForgeCacheDir   string
	ModulesCacheDir string
	EnvCacheDir     string
	Git             struct {
		privateKey string `yaml:"private_key"`
		username   string
	}
	Sources map[string]Source
	Timeout int `yaml:"timeout"`
}

// Source contains basic information about a Puppet environment repository
type Source struct {
	Remote     string
	Basedir    string
	Prefix     bool
	PrivateKey string `yaml:"private_key"`
}

// Puppetfile contains the key value pairs from the Puppetfile
type Puppetfile struct {
	moduleDir    string
	forgeModules map[string]ForgeModule
	gitModules   map[string]GitModule
	privateKey   string
}

// ForgeModule contains information (Version, Name, Author) about a Puppetlabs Forge module
type ForgeModule struct {
	version string
	name    string
	author  string
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
}

// ForgeResult is returned by queryForgeAPI and contains if and which version of the Puppetlabs Forge module needs to be downloaded
type ForgeResult struct {
	needToGet     bool
	versionNumber string
}

// ExecResult contains the exit code and output of an external command (e.g. git)
type ExecResult struct {
	returnCode int
	output     string
}

func main() {

	var (
		configFile    = flag.String("config", "", "which config file to use")
		envBranchFlag = flag.String("branch", "", "which git branch of the Puppet environment to update, e.g. core_foobar")
		pfFlag        = flag.Bool("puppetfile", false, "install all modules from Puppetfile in cwd")
		forceFlag     = flag.Bool("force", false, "purge the Puppet environment directory and do a full sync")
		dryRunFlag    = flag.Bool("dryrun", false, "do not modify anything, just print what would be changed")
		usemoveFlag   = flag.Bool("usemove", false, "do not use hardlinks to populate your Puppet environments with Puppetlabs Forge modules. Uses simple move instead of hard links and purge the Forge cache directory after each run!")
		debugFlag     = flag.Bool("debug", false, "log debug output, defaults to false")
		verboseFlag   = flag.Bool("verbose", false, "log verbose output, defaults to false")
		infoFlag      = flag.Bool("info", false, "log info output, defaults to false")
		versionFlag   = flag.Bool("version", false, "show build time and version number")
	)
	flag.Parse()

	debug = *debugFlag
	verbose = *verboseFlag
	info = *infoFlag
	force = *forceFlag
	dryRun = *dryRunFlag
	usemove = *usemoveFlag
	pfMode = *pfFlag

	if *versionFlag {
		fmt.Println("g10k Version 1.0 Build time:", buildtime, "UTC")
		os.Exit(0)
	}

	if len(os.Getenv("VIMRUNTIME")) > 0 {
		*configFile = "/home/andpaul/dev/go/src/github.com/xorpaul/g10k/test.yaml"
		*envBranchFlag = "invalid_modulename"
		debug = true
	}

	target := ""
	before := time.Now()
	if len(*configFile) > 0 {
		if usemove {
			log.Fatalln("Error: -usemove parameter is only allowed in -puppetfile mode!")
		}
		Debugf("Using as config file: " + *configFile)
		config = readConfigfile(*configFile)
		target = *configFile
		if len(*envBranchFlag) > 0 {
			resolvePuppetEnvironment(*envBranchFlag)
			target += " with branch " + *envBranchFlag
		} else {
			resolvePuppetEnvironment("")
		}
	} else {
		if pfMode {
			Debugf("Trying to use as Puppetfile: ./Puppetfile")
			sm := make(map[string]Source)
			sm["cmdlineparam"] = Source{Basedir: "."}
			cachedir := "/tmp/g10k"
			if len(os.Getenv("g10k_cachedir")) > 0 {
				cachedir = os.Getenv("g10k_cachedir")
				cachedir = checkDirAndCreate(cachedir, "cachedir environment variable g10k_cachedir")
				Debugf("Found environment variable g10k_cachedir set to: " + cachedir)
			} else {
				cachedir = checkDirAndCreate(cachedir, "cachedir default value")
			}
			config = ConfigSettings{CacheDir: cachedir, ForgeCacheDir: cachedir, ModulesCacheDir: cachedir, EnvCacheDir: cachedir, Sources: sm}
			target = "./Puppetfile"
			puppetfile := readPuppetfile("./Puppetfile", "")
			pfm := make(map[string]Puppetfile)
			pfm["cmdlineparam"] = puppetfile
			resolvePuppetfile(pfm)
		} else {
			log.Println("Error: no config file set")
			log.Printf("Example call: %s -config test.yaml\n", os.Args[0])
			log.Printf("or: %s -puppetfile\n", os.Args[0])
			os.Exit(1)
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

	fmt.Println("Synced", target, "with", syncGitCount, "git repositories and", syncForgeCount, "Forge modules in "+strconv.FormatFloat(time.Since(before).Seconds(), 'f', 1, 64)+"s with git ("+strconv.FormatFloat(syncGitTime, 'f', 1, 64)+"s sync, I/O", strconv.FormatFloat(cpGitTime, 'f', 1, 64)+"s) and Forge ("+strconv.FormatFloat(syncForgeTime, 'f', 1, 64)+"s query+download, I/O", strconv.FormatFloat(cpForgeTime, 'f', 1, 64)+"s)")
	if dryRun && (needSyncForgeCount > 0 || needSyncGitCount > 0) {
		os.Exit(1)
	}
}
