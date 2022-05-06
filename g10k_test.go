package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
)

func removeTimestampsFromDeployfile(file string) {
	if fileExists(file) {
		dr := readDeployResultFile(file)
		newDr := DeployResult{DeploySuccess: dr.DeploySuccess,
			Name:               dr.Name,
			Signature:          dr.Signature,
			PuppetfileChecksum: dr.PuppetfileChecksum,
			GitDir:             dr.GitDir,
			GitURL:             dr.GitURL,
		}

		writeStructJSONFile(file, newDr)

	}
}

func TestForgeChecksum(t *testing.T) {
	expectedFmm := ForgeModule{md5sum: "8a8c741978e578921e489774f05e9a65", fileSize: 57358}
	fmm := getMetadataForgeModule(ForgeModule{version: "2.2.0", name: "apt",
		author: "puppetlabs", baseURL: "https://forgeapi.puppet.com"})

	if fmm.md5sum != expectedFmm.md5sum {
		t.Error("Expected md5sum", expectedFmm.md5sum, "got", fmm.md5sum)
	}

	if fmm.fileSize != expectedFmm.fileSize {
		t.Error("Expected fileSize", expectedFmm.fileSize, "got", fmm.fileSize)
	}
}

func TestConfigPrefix(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile(filepath.Join("tests", funcName+".yaml"))

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-environment.git",
		Basedir: "/tmp/example", Prefix: "foobar", PrivateKey: "",
		AutoCorrectEnvironmentNames: "correct_and_warn"}

	expected := ConfigSettings{
		CacheDir: "/tmp/g10k", ForgeCacheDir: "/tmp/g10k/forge",
		ModulesCacheDir: "/tmp/g10k/modules", EnvCacheDir: "/tmp/g10k/environments",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppet.com"},
		Sources: s, Timeout: 5, Maxworker: 50, MaxExtractworker: 20,
		PurgeLevels: []string{"deployment", "puppetfile"}}

	if !reflect.DeepEqual(got, expected) {
		fmt.Println("### Expected:")
		spew.Dump(expected)
		fmt.Println("### Got:")
		spew.Dump(got)
		t.Errorf("Expected ConfigSettings: %+v, but got ConfigSettings: %+v", expected, got)
	}
}

func TestConfigForceForgeVersions(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile(filepath.Join("tests", funcName+".yaml"))

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-environment.git",
		Basedir: "/tmp/example", Prefix: "foobar", PrivateKey: "", ForceForgeVersions: true,
		WarnMissingBranch: false, AutoCorrectEnvironmentNames: "correct_and_warn"}

	expected := ConfigSettings{
		CacheDir: "/tmp/g10k", ForgeCacheDir: "/tmp/g10k/forge",
		ModulesCacheDir: "/tmp/g10k/modules", EnvCacheDir: "/tmp/g10k/environments",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppet.com"},
		Sources: s, Timeout: 5, Maxworker: 50, MaxExtractworker: 20,
		PurgeLevels: []string{"deployment", "puppetfile"}}

	if !reflect.DeepEqual(got, expected) {
		fmt.Println("### Expected:")
		spew.Dump(expected)
		fmt.Println("### Got:")
		spew.Dump(got)
		t.Errorf("Expected ConfigSettings: %+v, but got ConfigSettings: %+v", expected, got)
	}
}

func TestConfigAddWarning(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile(filepath.Join("tests", funcName+".yaml"))

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-environment.git",
		Basedir: "/tmp/example", PrivateKey: "", ForceForgeVersions: false,
		WarnMissingBranch: true, AutoCorrectEnvironmentNames: "correct_and_warn"}

	expected := ConfigSettings{
		CacheDir: "/tmp/g10k", ForgeCacheDir: "/tmp/g10k/forge",
		ModulesCacheDir: "/tmp/g10k/modules", EnvCacheDir: "/tmp/g10k/environments",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppet.com"},
		Sources: s, Timeout: 5, Maxworker: 50, MaxExtractworker: 20,
		PurgeLevels: []string{"deployment", "puppetfile"}}

	if !reflect.DeepEqual(got, expected) {
		fmt.Println("### Expected:")
		spew.Dump(expected)
		fmt.Println("### Got:")
		spew.Dump(got)
		t.Errorf("Expected ConfigSettings: %+v, but got ConfigSettings: %+v", expected, got)
	}
}

func TestConfigSimplePostrunCommand(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile(filepath.Join("tests", funcName+".yaml"))

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-environment.git",
		Basedir: "/tmp/example", PrivateKey: "", ForceForgeVersions: false,
		AutoCorrectEnvironmentNames: "correct_and_warn"}

	postrunCommand := []string{"/usr/bin/touch", "-f", "/tmp/g10kfoobar"}
	expected := ConfigSettings{
		CacheDir: "/tmp/g10k", ForgeCacheDir: "/tmp/g10k/forge",
		ModulesCacheDir: "/tmp/g10k/modules", EnvCacheDir: "/tmp/g10k/environments",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppet.com"},
		Sources: s, Timeout: 5, Maxworker: 50, MaxExtractworker: 20,
		PurgeLevels: []string{"deployment", "puppetfile"}, PostRunCommand: postrunCommand}

	if !reflect.DeepEqual(got, expected) {
		fmt.Println("### Expected:")
		spew.Dump(expected)
		fmt.Println("### Got:")
		spew.Dump(got)
		t.Errorf("Expected ConfigSettings: %+v, but got ConfigSettings: %+v", expected, got)
	}
}

func TestConfigPostrunCommand(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile(filepath.Join("tests", funcName+".yaml"))

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-test-environment.git",
		Basedir: "/tmp/example", PrivateKey: "", ForceForgeVersions: false, Prefix: "true",
		AutoCorrectEnvironmentNames: "correct_and_warn"}

	postrunCommand := []string{"tests/postrun.sh", "$modifiedenvs"}
	expected := ConfigSettings{
		CacheDir: "/tmp/g10k", ForgeCacheDir: "/tmp/g10k/forge",
		ModulesCacheDir: "/tmp/g10k/modules", EnvCacheDir: "/tmp/g10k/environments",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppet.com"},
		Sources: s, Timeout: 5, Maxworker: 50, MaxExtractworker: 20,
		PurgeLevels: []string{"deployment", "puppetfile"}, PostRunCommand: postrunCommand}

	if !reflect.DeepEqual(got, expected) {
		fmt.Println("### Expected:")
		spew.Dump(expected)
		fmt.Println("### Got:")
		spew.Dump(got)
		t.Errorf("Expected ConfigSettings: %+v, but got ConfigSettings: %+v", expected, got)
	}
}

func TestConfigDeploy(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile(filepath.Join("tests", funcName+".yaml"))

	s := make(map[string]Source)
	s["full"] = Source{Remote: "https://github.com/xorpaul/g10k-fullworking-env.git",
		Basedir: "/tmp/full", Prefix: "true", PrivateKey: "",
		AutoCorrectEnvironmentNames: "correct_and_warn"}

	expected := ConfigSettings{
		CacheDir: "/tmp/g10k", ForgeCacheDir: "/tmp/g10k/forge",
		ModulesCacheDir: "/tmp/g10k/modules", EnvCacheDir: "/tmp/g10k/environments",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppet.com"},
		Sources: s, Timeout: 5, Maxworker: 50, MaxExtractworker: 20,
		PurgeLevels:              []string{"deployment"},
		PurgeWhitelist:           []string{"custom.json", "**/*.xpp"},
		DeploymentPurgeWhitelist: []string{"full_hiera_*"}}

	if !reflect.DeepEqual(got, expected) {
		fmt.Println("### Expected:")
		spew.Dump(expected)
		fmt.Println("### Got:")
		spew.Dump(got)
		t.Errorf("Expected ConfigSettings: %+v, but got ConfigSettings: %+v", expected, got)
	}
}

func TestResolveConfigAddWarning(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigAddWarning.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "nonExistingBranch"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	if !strings.Contains(string(out), "WARNING: Couldn't find specified branch 'nonExistingBranch' anywhere in source 'example' (https://github.com/xorpaul/g10k-environment.git)") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
}

func TestResolveConfigAddError(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigAddError.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "nonExistingBranch"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 1 {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 1)
	}
	if !strings.Contains(string(out), "Couldn't find specified branch 'nonExistingBranch' anywhere in source 'example' (https://github.com/xorpaul/g10k-environment.git)") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
}

func TestResolveStatic(t *testing.T) {
	path, err := exec.LookPath("hashdeep")
	if err != nil {
		t.Skip("Skipping full Puppet environment resolve test, because package hashdeep is missing")
	}

	quiet = true
	purgeDir("./cache", "TestResolveStatic()")
	purgeDir("./example", "TestResolveStatic()")
	config = readConfigfile("tests/TestConfigStatic.yaml")
	// increase maxworker to finish the test quicker
	config.Maxworker = 500
	branchParam = "static"
	resolvePuppetEnvironment(false, "")

	// remove timestamps from .g10k-deploy.json otherwise hash sum would always differ
	removeTimestampsFromDeployfile("example/example_static/.g10k-deploy.json")

	cmd := exec.Command(path, "-vv", "-l", "-r", "-a", "-k", "tests/hashdeep_example_static.hashdeep", "./example")
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}
	if exitCode != 0 {
		t.Errorf("hashdeep terminated with %v, but we expected exit status 0\nOutput: %v", exitCode, string(out))
	}
	if !strings.Contains(string(out), "") {
		t.Errorf("resolvePuppetfile() terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
	Debugf("hashdeep output:" + string(out))

	purgeDir("example/example_static/external_modules/stdlib/spec/unit/facter/util", "TestResolveStatic()")

	cmd = exec.Command("hashdeep", "-l", "-r", "-a", "-k", "tests/hashdeep_example_static.hashdeep", "./example")
	out, err = cmd.CombinedOutput()
	exitCode = 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 1 {
		t.Errorf("hashdeep terminated with %v, but we expected exit status 1\nOutput: %v", exitCode, string(out))
	}

	fileMode, err := os.Stat("./example/example_static/external_modules/aws/examples/audit-security-groups/count_out_of_sync_resources.sh")
	if err != nil {
		t.Error("Error while trying to stat() testfile")
	}
	if fileMode.Mode().String() != "-rwxrwxr-x" {
		t.Error("Wrong file permission for test file. Check unTar()")
	}

}

func TestResolveStaticBlacklist(t *testing.T) {
	path, err := exec.LookPath("hashdeep")
	if err != nil {
		t.Skip("Skipping full Puppet environment resolve test, because package hashdeep is missing")
	}

	quiet = true
	purgeDir("./cache", "TestResolvStaticBlacklist()")
	purgeDir("./example", "TestResolvStaticBlacklist()")
	config = readConfigfile("tests/TestConfigStaticBlacklist.yaml")
	// increase maxworker to finish the test quicker
	config.Maxworker = 500
	branchParam = "blacklist"
	resolvePuppetEnvironment(false, "")

	// remove timestamps from .g10k-deploy.json otherwise hash sum would always differ
	removeTimestampsFromDeployfile("example/example_blacklist/.g10k-deploy.json")

	cmd := exec.Command(path, "-vv", "-l", "-r", "-a", "-k", "tests/hashdeep_example_static_blacklist.hashdeep", "./example")
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}
	if exitCode != 0 {
		t.Errorf("hashdeep terminated with %v, but we expected exit status 0\nOutput: %v", exitCode, string(out))
	}
	if !strings.Contains(string(out), "") {
		t.Errorf("resolvePuppetfile() terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
	Debugf("hashdeep output:" + string(out))

	expectedMissingFiles := []string{
		"example/example_blacklist/external_modules/stdlib/spec",
		"example/example_blacklist/external_modules/stdlib/readmes",
		"example/example_blacklist/external_modules/stdlib/examples",
	}
	for _, expectedMissingFile := range expectedMissingFiles {
		if fileExists(expectedMissingFile) {
			t.Errorf("blacklisted directory still exists that should have been purged! " + expectedMissingFile)
		}
	}

	purgeDir("example/example_blacklist/Puppetfile", "TestResolveStaticBlacklist()")

	cmd = exec.Command(path, "-l", "-r", "-a", "-k", "tests/hashdeep_example_static_blacklist.hashdeep", "./example")
	out, err = cmd.CombinedOutput()
	exitCode = 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 1 {
		t.Errorf("hashdeep terminated with %v, but we expected exit status 1\nOutput: %v", exitCode, string(out))
	}

}

func TestConfigGlobalAllowFail(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile(filepath.Join("tests", funcName+".yaml"))

	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	if !strings.Contains(string(out), "Failed to populate module /tmp/failing/master/modules/sensu but ignore-unreachable is set. Continuing...") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. Output was: %s", string(out))
	}
	debug = false
}

func TestInvalidFilesizeForgemodule(t *testing.T) {
	ts := spinUpFakeForge(t, "tests/fake-forge/invalid-filesize-puppetlabs-ntp-metadata.json")
	defer ts.Close()

	f := ForgeModule{version: "6.0.0", name: "ntp", author: "puppetlabs",
		baseURL: ts.URL, sha256sum: "59adaf8c4ab90ab629abcd8e965b6bdd28a022cf408e4e74b7294b47ce11644a"}
	fm := make(map[string]ForgeModule)
	fm["puppetlabs/ntp"] = f
	fmm := getMetadataForgeModule(fm["puppetlabs/ntp"])
	expectedFmm := ForgeModule{md5sum: "ccee7dd0c564de1c586be58dcf7626a5",
		fileSize: 1337}

	if fmm.md5sum != expectedFmm.md5sum {
		t.Error("Expected md5sum", expectedFmm.md5sum, "got", fmm.md5sum)
	}

	if fmm.fileSize != expectedFmm.fileSize {
		t.Error("Expected fileSize", expectedFmm.fileSize, "got", fmm.fileSize)
	}

	// fake Puppetlabs Forge looks good, continuing...
	fm["puppetlabs/ntp"] = f
	pf := Puppetfile{forgeModules: fm, source: "test",
		forgeBaseURL: f.baseURL, workDir: "/tmp/test_test"}
	pfm := make(map[string]Puppetfile)
	pfm["test"] = pf

	config = ConfigSettings{ForgeCacheDir: "/tmp/forge_cache", Maxworker: 500}
	defer purgeDir(pf.workDir, "TestInvalidMetadataForgemodule")
	defer purgeDir(config.ForgeCacheDir, "TestInvalidMetadataForgemodule")

	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		checkDirAndCreate(config.ForgeCacheDir, "TestInvalidMetadataForgemodule")
		resolvePuppetfile(pfm)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 1 {
		t.Errorf("resolvePuppetfile() terminated with %v, but we expected exit status 1", exitCode)
	}
	if !strings.Contains(string(out), "WARNING: calculated file size 760 for /tmp/forge_cache/puppetlabs-ntp-6.0.0.tar.gz does not match expected file size 1337") {
		t.Errorf("resolvePuppetfile() terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

}

func TestInvalidMd5sumForgemodule(t *testing.T) {
	ts := spinUpFakeForge(t, "tests/fake-forge/invalid-md5sum-puppetlabs-ntp-metadata.json")
	defer ts.Close()
	f := ForgeModule{version: "6.0.0", name: "ntp", author: "puppetlabs",
		baseURL: ts.URL, sha256sum: "a988a172a3edde6ac2a26d0e893faa88d37bc47465afc50d55225a036906c944"}
	fm := make(map[string]ForgeModule)
	fm["puppetlabs/ntp"] = f
	fmm := getMetadataForgeModule(fm["puppetlabs/ntp"])
	expectedFmm := ForgeModule{md5sum: "fakeMd5SumToCheckIfIntegrityCheckWorksAsExpected",
		fileSize: 760}

	if fmm.md5sum != expectedFmm.md5sum {
		t.Error("Expected md5sum", expectedFmm.md5sum, "got", fmm.md5sum)
	}

	if fmm.fileSize != expectedFmm.fileSize {
		t.Error("Expected fileSize", expectedFmm.fileSize, "got", fmm.fileSize)
	}

	// fake Puppetlabs Forge looks good, continuing...
	fm["puppetlabs/ntp"] = f
	pf := Puppetfile{forgeModules: fm, source: "test",
		forgeBaseURL: f.baseURL, workDir: "/tmp/test_test"}
	pfm := make(map[string]Puppetfile)
	pfm["test"] = pf

	config = ConfigSettings{ForgeCacheDir: "/tmp/forge_cache", Maxworker: 500}
	defer purgeDir(pf.workDir, "TestInvalidMd5sumForgemodule")
	defer purgeDir(config.ForgeCacheDir, "TestInvalidMd5sumForgemodule")

	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		checkDirAndCreate(config.ForgeCacheDir, "TestInvalidMd5sumForgemodule")
		resolvePuppetfile(pfm)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() && strings.Contains(string(out), "WARNING: calculated md5sum ccee7dd0c564de1c586be58dcf7626a5 for /tmp/forge_cache/puppetlabs-ntp-6.0.0.tar.gz does not match expected md5sum fakeMd5SumToCheckIfIntegrityCheckWorksAsExpected") {
		return
	}
	t.Errorf("resolvePuppetfile() terminated with %v, but we expected exit status 1", err)
}

func TestInvalidSha256sumForgemodule(t *testing.T) {
	ts := spinUpFakeForge(t, "tests/fake-forge/invalid-sha256sum-puppetlabs-ntp-metadata.json")
	defer ts.Close()
	f := ForgeModule{version: "6.0.0", name: "ntp", author: "puppetlabs",
		baseURL: ts.URL, sha256sum: "a988a172a3edde6ac2a26d0e893faa88d37bc47465afc50d55225a036906c944"}
	fm := make(map[string]ForgeModule)
	fm["puppetlabs/ntp"] = f
	fmm := getMetadataForgeModule(fm["puppetlabs/ntp"])
	expectedFmm := ForgeModule{md5sum: "ccee7dd0c564de1c586be58dcf7626a5",
		fileSize: 760}

	if fmm.md5sum != expectedFmm.md5sum {
		t.Error("Expected md5sum", expectedFmm.md5sum, "got", fmm.md5sum)
	}

	if fmm.fileSize != expectedFmm.fileSize {
		t.Error("Expected fileSize", expectedFmm.fileSize, "got", fmm.fileSize)
	}

	// fake Puppetlabs Forge looks good, continuing...
	fm["puppetlabs/ntp"] = f
	pf := Puppetfile{forgeModules: fm, source: "test",
		forgeBaseURL: f.baseURL, workDir: "/tmp/test_test"}
	pfm := make(map[string]Puppetfile)
	pfm["test"] = pf

	config = ConfigSettings{ForgeCacheDir: "/tmp/forge_cache", Maxworker: 500}
	defer purgeDir(pf.workDir, "TestInvalidMetadataForgemodule")
	defer purgeDir(config.ForgeCacheDir, "TestInvalidMetadataForgemodule")

	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		checkDirAndCreate(config.ForgeCacheDir, "TestInvalidSha256sumForgemodule")
		resolvePuppetfile(pfm)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() && strings.Contains(string(out), "WARNING: calculated sha256sum 59adaf8c4ab90ab629abcd8e965b6bdd28a022cf408e4e74b7294b47ce11644a for /tmp/forge_cache/puppetlabs-ntp-6.0.0.tar.gz does not match expected sha256sum a988a172a3edde6ac2a26d0e893faa88d37bc47465afc50d55225a036906c944") {
		return
	}
	t.Errorf("resolvePuppetfile() terminated with %v, but we expected exit status 1", err)
}

// TODO add TestMissingVersionInForgeAPI

func spinUpFakeForge(t *testing.T, metadataFile string) *httptest.Server {
	// spin up HTTP test server to serve fake/invalid Forge module metadata
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/releases/puppetlabs-ntp-6.0.0" {
			body, err := ioutil.ReadFile(metadataFile)
			if err != nil {
				t.Error(err)
			}
			fmt.Fprint(w, string(body))
		} else if r.URL.Path == "/v3/modules/puppetlabs-ntp" {
			body, err := ioutil.ReadFile("tests/fake-forge/latest-puppetlabs-ntp-metadata.json")
			if err != nil {
				t.Error(err)
			}
			fmt.Fprint(w, string(body))
		} else if r.URL.Path == "/v3/files/puppetlabs-ntp-6.0.0.tar.gz" {
			body, err := ioutil.ReadFile("tests/fake-forge/fake-puppetlabs-ntp-6.0.0.tar.gz")
			if err != nil {
				t.Error(err)
			}
			fmt.Fprint(w, string(body))
		} else {
			t.Error("Unexpected request URL:" + r.URL.Path)
		}
	}))
	return ts

}

func TestModuleDirOverride(t *testing.T) {
	got := readPuppetfile("tests/TestReadPuppetfile", "", "test", "test", false, false)
	//fmt.Println(got.forgeModules["apt"].moduleDir)
	if got.forgeModules["apt"].moduleDir != "external_modules" {
		t.Error("Expected 'external_modules' for module dir, but got", got.forgeModules["apt"].moduleDir)
	}
	if got.gitModules["another_module"].moduleDir != "modules" {
		t.Error("Expected 'modules' for module dir, but got", got.gitModules["another_module"].moduleDir)
	}
	moduleDirParam = "foobar"
	got = readPuppetfile("tests/TestReadPuppetfile", "", "test", "test", false, false)
	if got.forgeModules["apt"].moduleDir != "foobar" {
		t.Error("Expected '", moduleDirParam, "' for module dir, but got", got.forgeModules["apt"].moduleDir)
	}
	moduleDirParam = ""
}

func TestResolveConfigExitIfUnreachable(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigExitIfUnreachable.yaml")
	purgeDir(config.CacheDir, "TestResolveConfigExitIfUnreachable()")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "single"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 1 {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 1)
	}
	//fmt.Println(string(out))
	if !strings.Contains(string(out), "WARN: git repository git://github.com/xorpaul/g10k-environment-unavailable.git does not exist or is unreachable at this moment!") || !strings.Contains(string(out), "WARNING: Could not resolve git repository in source 'example' (git://github.com/xorpaul/g10k-environment-unavailable.git)") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
}

func TestResolveConfigExitIfUnreachableFalse(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigExitIfUnreachableFalse.yaml")
	purgeDir(config.CacheDir, "TestResolveConfigExitIfUnreachableFalse()")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "single"

		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	if !strings.Contains(string(out), "WARN: git repository git://github.com/xorpaul/g10k-environment-unavailable.git does not exist or is unreachable at this moment!") || !strings.Contains(string(out), "WARNING: Could not resolve git repository in source 'example' (git://github.com/xorpaul/g10k-environment-unavailable.git)") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
}

func TestConfigUseCacheFallback(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile(filepath.Join("tests", funcName+".yaml"))
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "single_fail"
		resolvePuppetEnvironment(false, "")
		return
	}

	// get the module to cache it
	gm := GitModule{}
	gm.git = "https://github.com/puppetlabs/puppetlabs-firewall.git"
	doMirrorOrUpdate(gm, "/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git", 0)

	// rename the cached module dir to match the otherwise failing single_fail env
	unresolvableGitDir := "/tmp/g10k/modules/https-__.com_puppetlabs_puppetlabs-firewall.git"
	purgeDir(unresolvableGitDir, funcName)
	purgeDir("/tmp/example/single_fail", funcName)
	err := os.Rename("/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git", unresolvableGitDir)
	if err != nil {
		t.Error(err)
	}

	// change the git remote url to something that does not resolve https://.com/...
	er := executeCommand("git --git-dir "+unresolvableGitDir+" remote set-url origin https://.com/puppetlabs/puppetlabs-firewall.git", 5, false)
	if er.returnCode != 0 {
		t.Error("Rewriting the git remote url of " + unresolvableGitDir + " to https://.com/puppetlabs/puppetlabs-firewall.git failed! Errorcode: " + strconv.Itoa(er.returnCode) + "Error: " + er.output)
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}
	//fmt.Println(string(out))

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	if !strings.Contains(string(out), "WARN: git repository https://.com/puppetlabs/puppetlabs-firewall.git does not exist or is unreachable at this moment!") || !strings.Contains(string(out), "WARN: Trying to use cache for https://.com/puppetlabs/puppetlabs-firewall.git git repository") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
	if !fileExists("/tmp/example/single_fail/modules/firewall/metadata.json") {
		t.Errorf("terminated with the correct exit code and the correct output, but the resulting module was missing")
	}
}

func TestConfigUseCacheFallbackFalse(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile(filepath.Join("tests", funcName+".yaml"))
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "single_fail"
		resolvePuppetEnvironment(false, "")
		return
	}

	// get the module to cache it
	gm := GitModule{}
	gm.git = "https://github.com/puppetlabs/puppetlabs-firewall.git"
	doMirrorOrUpdate(gm, "/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git", 0)

	// rename the cached module dir to match the otherwise failing single_fail env
	unresolvableGitDir := "/tmp/g10k/modules/https-__.com_puppetlabs_puppetlabs-firewall.git"
	purgeDir(unresolvableGitDir, funcName)
	purgeDir("/tmp/example/single_fail", funcName)
	err := os.Rename("/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git", unresolvableGitDir)
	if err != nil {
		t.Error(err)
	}

	// change the git remote url to something that does not resolve https://.com/...
	er := executeCommand("git --git-dir "+unresolvableGitDir+" remote set-url origin https://.com/puppetlabs/puppetlabs-firewall.git", 5, false)
	if er.returnCode != 0 {
		t.Error("Rewriting the git remote url of " + unresolvableGitDir + " to https://.com/puppetlabs/puppetlabs-firewall.git failed! Errorcode: " + strconv.Itoa(er.returnCode) + "Error: " + er.output)
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 1 {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 1)
	}
	//fmt.Println(string(out))
	if !strings.Contains(string(out), "WARN: git repository https://.com/puppetlabs/puppetlabs-firewall.git does not exist or is unreachable at this moment!") || !strings.Contains(string(out), "Fatal: Failed to clone or pull https://.com/puppetlabs/puppetlabs-firewall.git") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
	if fileExists("/tmp/example/single_fail/modules/firewall/metadata.json") {
		t.Errorf("terminated with the correct exit code and the correct output, but the resulting module was not missing")
	}
}

func TestReadPuppetfileUseCacheFallback(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigUseCacheFallback.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = "single_fail_forge"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	fm := ForgeModule{version: "1.9.0", author: "puppetlabs", name: "firewall"}
	config.Forge.Baseurl = "https://forgeapi.puppet.com"
	downloadForgeModule("puppetlabs-firewall", "1.9.0", fm, 1)

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}

	expectedLines := []string{
		"Forge API error, trying to use cache for module puppetlabs/puppetlabs-firewall",
		"Using cached version 1.9.0 for puppetlabs-firewall-latest",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("terminated with the correct exit code, but the expected line '"+expectedLine+"' was missing\n. out: %s", string(out))
		}
	}

	//fmt.Println(string(out))
	if !fileExists("/tmp/example/single_fail_forge/modules/firewall/metadata.json") {
		t.Errorf("terminated with the correct exit code and the correct output, but the resulting module was missing")
	}
}

func TestReadPuppetfileUseCacheFallbackFalse(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigUseCacheFallback.yaml")
	purgeDir("/tmp/example", funcName)
	purgeDir(config.ForgeCacheDir, funcName)
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "single_fail_forge"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 1 {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 1)
	}
	//fmt.Println(string(out))
	if !strings.Contains(string(out), "Forge API error, trying to use cache for module puppetlabs/puppetlabs-firewall\nCould not find any cached version for Forge module puppetlabs-firewall") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
	if fileExists("/tmp/example/single_fail_forge/modules/firewall/metadata.json") {
		t.Errorf("terminated with the correct exit code and the correct output, but the resulting module was not missing")
	}
}

func TestResolvePuppetfileInstallPath(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigUseCacheFallback.yaml")
	purgeDir("/tmp/example", funcName)
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "install_path"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))
	metadataFile := "/tmp/example/install_path/modules/sensu/metadata.json"
	if !fileExists(metadataFile) {
		t.Errorf("terminated with the correct exit code, but the resulting module was missing %s", metadataFile)
	}

	metadata := readModuleMetadata(metadataFile)
	//fmt.Println(metadata)
	if metadata.version != "2.0.0" {
		t.Errorf("terminated with the correct exit code, but the resolved metadata.json is unexpected %s", metadataFile)
	}

	metadataFile2 := "/tmp/example/install_path/modules/external/apt/metadata.json"
	if !fileExists(metadataFile2) {
		t.Errorf("terminated with the correct exit code, but the resulting module was missing %s", metadataFile2)
	}
}

func TestResolvePuppetfileInstallPathTwice(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigUseCacheFallback.yaml")
	purgeDir("/tmp/example", funcName)
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "install_path"
		resolvePuppetEnvironment(false, "")
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))
	metadataFile := "/tmp/example/install_path/modules/sensu/metadata.json"
	if !fileExists(metadataFile) {
		t.Errorf("terminated with the correct exit code, but the resulting module was missing %s", metadataFile)
	}

	metadata := readModuleMetadata(metadataFile)
	//fmt.Println(metadata)
	if metadata.version != "2.0.0" {
		t.Errorf("terminated with the correct exit code, but the resolved metadata.json is unexpected %s", metadataFile)
	}

	metadataFile2 := "/tmp/example/install_path/modules/external/apt/metadata.json"
	if !fileExists(metadataFile2) {
		t.Errorf("terminated with the correct exit code, but the resulting module was missing %s", metadataFile2)
	}
}

func TestResolvePuppetfileSingleModuleForge(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigUseCacheFallback.yaml")
	sensuDir := "/tmp/example/single_module/modules/sensu"
	metadataFile := sensuDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		moduleParam = "stdlib"
		//debug = true
		branchParam = "single_module"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	branchParam = "single_module"
	resolvePuppetEnvironment(false, "")
	if !fileExists(metadataFile) {
		t.Errorf("terminated with the correct exit code, but the resolved metadata.json is missing %s", metadataFile)
	}
	purgeDir(sensuDir, funcName)
	if fileExists(metadataFile) {
		t.Errorf("error while purging directory with file %s", metadataFile)
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	moduleParam = "stdlib"
	if fileExists(metadataFile) {
		t.Errorf("error found file %s of a module that should not be there, because -module is set to %s", metadataFile, moduleParam)
	}

	if !fileExists(strings.Replace(metadataFile, "sensu", "firewall", -1)) {
		t.Errorf("error missing file %s of a module that should be there, despite -module being set to %s", strings.Replace(metadataFile, "sensu", "firewall", -1), moduleParam)
	}

	if !fileExists(strings.Replace(metadataFile, "sensu", "concat", -1)) {
		t.Errorf("error missing file %s of a module that should be there, despite -module being set to %s", strings.Replace(metadataFile, "sensu", "concat", -1), moduleParam)
	}

	moduleParam = ""
}

func TestResolvePuppetfileSingleModuleGit(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigUseCacheFallback.yaml")
	concatDir := "/tmp/example/single_module/modules/concat"
	metadataFile := concatDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		moduleParam = "firewall"
		//debug = true
		branchParam = "single_module"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	branchParam = "single_module"
	resolvePuppetEnvironment(false, "")
	if !fileExists(metadataFile) {
		t.Errorf("expected module metadata.json is missing %s", metadataFile)
	}
	purgeDir(concatDir, funcName)
	if fileExists(metadataFile) {
		t.Errorf("error while purging directory with file %s", metadataFile)
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	moduleParam = "firewall"
	if fileExists(metadataFile) {
		t.Errorf("error found file %s of a module that should not be there, because -module is set to %s", metadataFile, moduleParam)
	}

	if !fileExists(strings.Replace(metadataFile, "concat", "stdlib", -1)) {
		t.Errorf("error missing file %s of a module that should be there, despite -module being set to %s", strings.Replace(metadataFile, "concat", "concat", -1), moduleParam)
	}

	if !fileExists(strings.Replace(metadataFile, "concat", "sensu", -1)) {
		t.Errorf("error missing file %s of a module that should be there, despite -module being set to %s", strings.Replace(metadataFile, "concat", "concat", -1), moduleParam)
	}
	moduleParam = ""

}

func TestResolvePuppetfileFallback(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrefix.yaml")
	aptDir := "/tmp/example/foobar_fallback/modules/apt"
	metadataFile := aptDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = "fallback"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	branchParam = "fallback"
	resolvePuppetEnvironment(false, "")
	if !fileExists(metadataFile) {
		t.Errorf("expected module metadata.json is missing %s", metadataFile)
	}
	purgeDir(aptDir, funcName)
	if fileExists(metadataFile) {
		t.Errorf("error while purging directory with file %s", metadataFile)
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	if !strings.Contains(string(out), "Trying to resolve /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git with branch noooopee") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !strings.Contains(string(out), "executeCommand(): Executing git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git rev-parse --verify 'foooooobbaar^{object}'") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !fileExists(metadataFile) {
		t.Errorf("error missing file %s", metadataFile)
	}

	moduleParam = ""
	debug = false

}

func TestResolvePuppetfileDefaultBranch(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrefix.yaml")
	apacheDir := "/tmp/example/foobar_default_branch/modules/apache"
	metadataFile := apacheDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = "default_branch"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	branchParam = "default_branch"
	resolvePuppetEnvironment(false, "")
	if !fileExists(metadataFile) {
		t.Errorf("expected module metadata.json is missing %s", metadataFile)
	}
	purgeDir(apacheDir, funcName)
	if fileExists(metadataFile) {
		t.Errorf("error while purging directory with file %s", metadataFile)
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	if !strings.Contains(string(out), "Trying to resolve /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apache.git with branch default_branch") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !strings.Contains(string(out), "Executing git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apache.git rev-parse --verify 'main^{object}' took") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !fileExists(metadataFile) {
		t.Errorf("error missing file %s", metadataFile)
	}

	moduleParam = ""
	debug = false

}

func TestResolvePuppetfileControlBranch(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrefix.yaml")
	testDir := "/tmp/example/foobar_control_branch_foobar/modules/g10k_testmodule"
	initFile := filepath.Join(testDir, "manifests/init.pp")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = "control_branch_foobar"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	branchParam = "control_branch_foobar"
	resolvePuppetEnvironment(false, "")
	if !fileExists(initFile) {
		t.Errorf("expected module init.pp is missing %s", initFile)
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	if !strings.Contains(string(out), "Trying to resolve /tmp/g10k/modules/https-__github.com_xorpaul_g10k_testmodule.git with branch control_branch_foobar") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !strings.Contains(string(out), "Executing git --git-dir /tmp/g10k/modules/https-__github.com_xorpaul_g10k_testmodule.git rev-parse --verify 'control_branch_foobar^{object}' took") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	branchFile := filepath.Join(testDir, "MODULEBRANCHNAME_IS_control_branch_foobar")
	if !fileExists(branchFile) {
		t.Errorf("error missing file %s, which means that not the correct module branch was used by :control_branch", branchFile)
	}

	moduleParam = ""
	debug = false

}

func TestResolvePuppetfileControlBranchDefault(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrefix.yaml")
	apacheDir := "/tmp/example/foobar_control_branch_default/modules/apache"
	metadataFile := filepath.Join(apacheDir, "metadata.json")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = "control_branch_default"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	branchParam = "control_branch_default"
	resolvePuppetEnvironment(false, "")
	if !fileExists(metadataFile) {
		t.Errorf("expected module metadata.json is missing %s", metadataFile)
	}
	purgeDir(apacheDir, funcName)
	if fileExists(metadataFile) {
		t.Errorf("error while purging directory with file %s", metadataFile)
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	if !strings.Contains(string(out), "Trying to resolve /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apache.git with branch control_branch_default") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !strings.Contains(string(out), "Executing git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apache.git rev-parse --verify 'main^{object}' took") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !fileExists(metadataFile) {
		t.Errorf("error missing file %s", metadataFile)
	}

	moduleParam = ""
	debug = false

}

func TestConfigRetryGitCommands(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile(filepath.Join("tests", funcName+".yaml"))
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "single_git"
		resolvePuppetEnvironment(false, "")
		return
	}

	localGitRepoDir := "/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git"
	purgeDir(localGitRepoDir, funcName)

	// get the module to cache it
	gm := GitModule{}
	gm.git = "https://github.com/puppetlabs/puppetlabs-firewall.git"
	doMirrorOrUpdate(gm, localGitRepoDir, 0)

	// corrupt the local git module repository
	purgeDir(filepath.Join(localGitRepoDir, "objects"), "corrupt local git repository for TestConfigRetryGitCommands")

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	//fmt.Println(string(out))
	if !strings.Contains(string(out), "WARN: git command failed: git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git remote update --prune deleting local cached repository and retrying...") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
	if !fileExists("/tmp/example/single_git/modules/firewall/metadata.json") {
		t.Errorf("terminated with the correct exit code and the correct output, but the resulting module was missing")
	}
}

func TestConfigRetryGitCommandsFail(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile(filepath.Join("tests", "TestConfigRetryGitCommands.yaml"))
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = "invalid_git_object"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	//fmt.Println(string(out))
	expectedExitCode := 1
	if expectedExitCode != exitCode {
		t.Fatalf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	if !strings.Contains(string(out), "Failed to resolve git module 'firewall' with repository https://github.com/puppetlabs/puppetlabs-firewall.git and branch/reference '0000000000000000000000000000000000000000' used in control repository branch 'invalid_git_object' or Puppet environment 'invalid_git_object'") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
	//if !fileExists("/tmp/example/single_fail/modules/firewall/metadata.json") {
	//	t.Errorf("terminated with the correct exit code and the correct output, but the resulting module was missing")
	//}
}

func TestResolvePuppetfileLocalModules(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrefix.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		purgeDir("/tmp/example/", funcName)
		debug = true
		branchParam = "local_modules"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"Need to sync /tmp/example/foobar_local_modules/modules/stdlib",
		"Not deleting /tmp/example/foobar_local_modules/modules/localstuff as it is declared as a local module",
		"Not deleting /tmp/example/foobar_local_modules/modules/localstuff2 as it is declared as a local module",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	file1 := "/tmp/example/foobar_local_modules/modules/localstuff/foobar3"
	if !fileExists(file1) {
		t.Errorf("error missing file %s", file1)
	}

	file2 := "/tmp/example/foobar_local_modules/modules/localstuff2/foobar"
	if !fileExists(file2) {
		t.Errorf("error missing file %s", file2)
	}

	moduleParam = ""
	debug = false

}

func TestResolvePuppetfileInvalidGitObject(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrefix.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = "invalid_git_object"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	//fmt.Println(string(out))
	if exitCode != 1 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 1, string(out))
	}

	expectingString := "Failed to resolve git module 'firewall' with repository https://github.com/puppetlabs/puppetlabs-firewall.git and branch/reference '0000000000000000000000000000000000000000' used in control repository branch 'invalid_git_object' or Puppet environment 'foobar_invalid_git_object'"
	if !strings.Contains(string(out), expectingString) {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s\nExpecting string: %s", string(out), expectingString)
	}

	moduleParam = ""
	debug = false

}

func TestUnTarPreserveTimestamp(t *testing.T) {
	purgeDir("/tmp/example", "TestUnTarPreserveTimestamp()")
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrefix.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = "master"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}

	gitFile := "/tmp/example/foobar_master/external_modules/apt/metadata.json"
	if fileExists(gitFile) {
		if fileInfo, err := os.Stat(gitFile); err == nil {
			//fmt.Println("fileInfo", fileInfo.ModTime())
			if fileInfo.ModTime().Before(time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)) {
				t.Errorf("ModTime of file %s is incorrect: %s", gitFile, fileInfo.ModTime())
			}
		}
	} else {
		t.Errorf("error missing file %s", gitFile)
	}

	forgeFile := "/tmp/example/foobar_master/external_modules/stdlib/metadata.json"
	if fileExists(forgeFile) {
		if fileInfo, err := os.Stat(forgeFile); err == nil {
			//fmt.Println("fileInfo", fileInfo.ModTime())
			if fileInfo.ModTime().Before(time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)) {
				t.Errorf("ModTime of file %s is incorrect: %s", forgeFile, fileInfo.ModTime())
			}
		}
	} else {
		t.Errorf("error missing file %s", forgeFile)
	}
}

func TestSupportOldGitWithoutObjectSyntax(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigGitObjectSyntaxNotSupported.yaml")
	aptDir := "/tmp/example/foobar_fallback/modules/apt"
	metadataFile := aptDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = "fallback"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	branchParam = "fallback"
	resolvePuppetEnvironment(false, "")
	if !fileExists(metadataFile) {
		t.Errorf("expected module metadata.json is missing %s", metadataFile)
	}
	purgeDir(aptDir, funcName)
	if fileExists(metadataFile) {
		t.Errorf("error while purging directory with file %s", metadataFile)
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	if !strings.Contains(string(out), "Trying to resolve /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git with branch noooopee") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !strings.Contains(string(out), "executeCommand(): Executing git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git rev-parse --verify 'foooooobbaar'") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !fileExists(metadataFile) {
		t.Errorf("error missing file %s", metadataFile)
	}

	moduleParam = ""
	debug = false

}

func TestSupportOldGitWithoutObjectSyntaxParameter(t *testing.T) {
	quiet = true
	gitObjectSyntaxNotSupported = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrefix.yaml")
	aptDir := "/tmp/example/foobar_fallback/modules/apt"
	metadataFile := aptDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = "fallback"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	branchParam = "fallback"
	resolvePuppetEnvironment(false, "")
	if !fileExists(metadataFile) {
		t.Errorf("expected module metadata.json is missing %s", metadataFile)
	}
	purgeDir(aptDir, funcName)
	if fileExists(metadataFile) {
		t.Errorf("error while purging directory with file %s", metadataFile)
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	if !strings.Contains(string(out), "Trying to resolve /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git with branch noooopee") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !strings.Contains(string(out), "executeCommand(): Executing git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git rev-parse --verify 'foooooobbaar'") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !fileExists(metadataFile) {
		t.Errorf("error missing file %s", metadataFile)
	}

	moduleParam = ""
	debug = false

}

func TestAutoCorrectEnvironmentNamesDefault(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile(filepath.Join("tests", funcName+".yaml"))
	firewallDir := "/tmp/example/single_autocorrect___fooo/modules/firewall"
	metadataFile := firewallDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "single_autocorrect-%-fooo"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}

	if !strings.Contains(string(out), "Renaming branch single_autocorrect-%-fooo to single_autocorrect___fooo") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !fileExists(metadataFile) {
		t.Errorf("expected module metadata.json is missing %s", metadataFile)
	}

	purgeDir("/tmp/example", funcName)
	moduleParam = ""
	debug = false

}

func TestAutoCorrectEnvironmentNamesWarn(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile(filepath.Join("tests", funcName+".yaml"))
	firewallDir := "/tmp/example/single_autocorrect___fooo/modules/firewall"
	metadataFile := firewallDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "single_autocorrect-%-fooo"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}

	if !strings.Contains(string(out), "Renaming branch single_autocorrect-%-fooo to single_autocorrect___fooo") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !fileExists(metadataFile) {
		t.Errorf("expected module metadata.json is missing %s", metadataFile)
	}

	purgeDir("/tmp/example", funcName)
	moduleParam = ""
	debug = false

}

func TestAutoCorrectEnvironmentNamesError(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile(filepath.Join("tests", funcName+".yaml"))
	firewallDir := "/tmp/example/single_autocorrect-%-fooo/modules/firewall"
	metadataFile := firewallDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "single_autocorrect-%-fooo"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}

	if !strings.Contains(string(out), "Ignoring branch single_autocorrect-%-fooo, because it contains invalid characters") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if fileExists(metadataFile) {
		t.Errorf("branch with invalid characters exists, which should have been skipped: %s", metadataFile)
	}

	purgeDir("/tmp/example", funcName)
	moduleParam = ""
	debug = false
}

func TestLastCheckedFile(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrefix.yaml")
	lastCheckedFile := "/tmp/g10k/forge/puppetlabs-inifile-latest-last-checked"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "single_cache"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}

	if !fileExists(lastCheckedFile) {
		t.Errorf("Forge cache file missing: %s", lastCheckedFile)
	}

	fm := ForgeModule{version: "latest", name: "inifile", author: "puppetlabs", fileSize: 0, cacheTTL: 0}
	json, _ := ioutil.ReadFile(lastCheckedFile)
	latestForgeModules.m = make(map[string]string)

	result := parseForgeAPIResult(string(json), fm)
	result2 := queryForgeAPI(fm)

	if !equalForgeResult(result, result2) {
		t.Errorf("Forge result is not the same! a: %v b: %v", result, result2)
	}

	// in some older g10k versions the -latest-last-checked file was just empty and
	// did not contain the JSON Forge API response
	// So truncate the file and check the contents again

	// skip err as we explicitly checked for it above
	f, _ := os.Create(lastCheckedFile)
	f.WriteString("")
	f.Close()
	f.Sync()
	fi, _ := os.Stat(lastCheckedFile)
	if fi.Size() != 0 {
		t.Errorf("Forge cache file could not be truncated/emptied: %s", lastCheckedFile)
	}

	branchParam = "single_cache"
	resolvePuppetEnvironment(false, "")
	json, _ = ioutil.ReadFile(lastCheckedFile)
	result = parseForgeAPIResult(string(json), fm)
	result2 = queryForgeAPI(fm)

	if !equalForgeResult(result, result2) {
		t.Errorf("Forge result is not the same! a: %v b: %v", result, result2)
	}

	purgeDir("/tmp/example", funcName)
	purgeDir("/tmp/g10k", funcName)
	moduleParam = ""
	debug = false
}

func TestSimplePostrunCommand(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigSimplePostrunCommand.yaml")

	touchFile := "/tmp/g10kfoobar"
	purgeDir(touchFile, funcName)
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		branchParam = "single"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}

	checkForAndExecutePostrunCommand()

	if !fileExists(touchFile) {
		t.Errorf("postrun created file missing: %s", touchFile)
	}

	purgeDir("/tmp/example", funcName)
	purgeDir("/tmp/g10k", funcName)
	moduleParam = ""
	debug = false
}

func TestPostrunCommand(t *testing.T) {
	needSyncDirs = append(needSyncDirs, "")
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPostrunCommand.yaml")

	postrunLogfile := "/tmp/postrun.log"
	purgeDir(postrunLogfile, funcName)
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		checkForAndExecutePostrunCommand()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}

	if !fileExists(postrunLogfile) {
		t.Errorf("postrun logfile file missing: %s", postrunLogfile)
	}

	content, _ := ioutil.ReadFile(postrunLogfile)

	expectedLines := []string{
		"postrun command wrapper script received argument: example_master",
		"postrun command wrapper script received argument: example_foobar",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(content), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in postrun logfile " + postrunLogfile + " Check variable replacement in postrun command.")
		}
	}

	purgeDir("/tmp/example", funcName)
	purgeDir("/tmp/g10k", funcName)
	moduleParam = ""
	debug = false
}

func TestPostrunCommandDirs(t *testing.T) {
	needSyncDirs = append(needSyncDirs, "")
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPostrunCommandDirs.yaml")

	postrunLogfile := "/tmp/postrun.log"
	purgeDir(postrunLogfile, funcName)
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		checkForAndExecutePostrunCommand()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}

	if !fileExists(postrunLogfile) {
		t.Errorf("postrun logfile file missing: %s", postrunLogfile)
	}

	content, _ := ioutil.ReadFile(postrunLogfile)

	expectedLines := []string{
		"postrun command wrapper script received argument: /tmp/example/example_master",
		"postrun command wrapper script received argument: /tmp/example/example_foobar",
		"postrun command wrapper script received argument: /tmp/example/example_foobar/modules/systemd",
		"postrun command wrapper script received argument: /tmp/example/example_master/modules/systemd",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(content), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in postrun logfile " + postrunLogfile + ". Check variable replacement in postrun command.")
		}
	}

	purgeDir("/tmp/example", funcName)
	purgeDir("/tmp/g10k", funcName)
	moduleParam = ""
	debug = false
}

func TestMultipleModuledirs(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrefix.yaml")
	moduleDir1File := "/tmp/example/foobar_multiple_moduledir/external_modules/stdlib/metadata.json"
	moduleDir2File := "/tmp/example/foobar_multiple_moduledir/base_modules/apt/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "multiple_moduledir"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}

	if !fileExists(moduleDir1File) {
		t.Errorf("Module file in moduledir 1 missing: %s", moduleDir1File)
	}

	if !fileExists(moduleDir2File) {
		t.Errorf("Module file in moduledir 2 missing: %s", moduleDir2File)
	}

	unmanagedModule1 := "/tmp/example/foobar_multiple_moduledir/external_modules/foo"
	unmanagedModule2 := "/tmp/example/foobar_multiple_moduledir/base_modules/bar"
	checkDirAndCreate(unmanagedModule1, funcName)
	checkDirAndCreate(unmanagedModule2, funcName)

	branchParam = "multiple_moduledir"
	resolvePuppetEnvironment(false, "")

	if isDir(unmanagedModule1) {
		t.Errorf("Unmanaged Module directory 1 is still there and should not be: %s", unmanagedModule1)
	}

	if isDir(unmanagedModule2) {
		t.Errorf("Unmanaged Module directory 2 is still there and should not be: %s", unmanagedModule2)
	}

	//purgeDir("/tmp/example", funcName)
	purgeDir("/tmp/g10k", funcName)
	moduleParam = ""
	branchParam = ""
	debug = false
}

func TestFailedGit(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigRetryGitCommands.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		branchParam = "single_fail"
		resolvePuppetEnvironment(false, "")
		return
	}

	// get the module to cache it
	gitDir := "/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git"
	gm := GitModule{}
	gm.git = "https://github.com/puppetlabs/puppetlabs-firewall.git"
	purgeDir(gitDir, funcName)
	doMirrorOrUpdate(gm, gitDir, 0)

	// change the git remote url to something that does not resolve https://.com/...
	er := executeCommand("git --git-dir "+gitDir+" remote set-url origin https://.com/puppetlabs/puppetlabs-firewall.git", 5, false)
	if er.returnCode != 0 {
		t.Error("Rewriting the git remote url of " + gitDir + " to https://.com/puppetlabs/puppetlabs-firewall.git failed! Errorcode: " + strconv.Itoa(er.returnCode) + "Error: " + er.output)
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 1 {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	//fmt.Println(string(out))
	if !strings.Contains(string(out), "WARN: git command failed: git clone --mirror https://.com/puppetlabs/puppetlabs-firewall.git /tmp/g10k/modules/https-__.com_puppetlabs_puppetlabs-firewall.git deleting local cached repository and retrying...") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
	purgeDir("/tmp/example", funcName)
}

func TestCheckDirPermissions(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	cacheDir := "/tmp/g10k"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		config = readConfigfile("tests/TestConfigPrefix.yaml")
		branchParam = "single"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir(cacheDir, funcName)
	// create cacheDir and make sure the cachedir does not have write permissions
	if err := os.MkdirAll(cacheDir, 0444); err != nil {
		Fatalf("checkDirAndCreate(): Error: failed to create directory: " + cacheDir + " Error: " + err.Error())
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 1
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))
	if !strings.Contains(string(out), "checkDirAndCreate(): Error: /tmp/g10k exists, but is not writable! Exiting!") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
	if err := os.Chmod(cacheDir, 0777); err != nil {
		t.Errorf("Could not add write permissions again for cachedir: " + cacheDir + " Error: " + err.Error())
	}
	purgeDir(cacheDir, funcName)
	purgeDir("/tmp/example", funcName)
}

func TestPurgeWhitelist(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	cacheDir := "/tmp/g10k"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		config = readConfigfile("tests/TestConfigExamplePurgeEnvironment.yaml")
		branchParam = "single_git"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	createOrPurgeDir("/tmp/example/single_git/stale_directory_that_should_be_purged", funcName)
	createOrPurgeDir("/tmp/example/single_git/.resource_types", funcName)
	f, _ := os.Create("/tmp/example/single_git/.latest_revision")
	defer f.Close()
	f.WriteString("foobar")
	f.Sync()
	frt, err := os.Create("/tmp/example/single_git/.resource_types/foobar.pp")
	if err != nil {
		t.Errorf("Error while creating test file")
	}
	defer frt.Close()
	frt.WriteString("fake resource type")
	frt.Sync()
	createOrPurgeDir("/tmp/example/single_git/modules/", funcName)
	createOrPurgeDir("/tmp/example/single_git/modules/firewall/", funcName)
	createOrPurgeDir("/tmp/example/single_git/modules/firewall/manifests/", funcName)
	fpf, _ := os.Create("/tmp/example/single_git/modules/firewall/manifests/stale.pp")
	defer fpf.Close()
	fpf.WriteString("fake stale module file")
	fpf.Sync()

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"Removing unmanaged path /tmp/example/single_git/stale_directory_that_should_be_purged",
		"DEBUG purgeDir(): Trying to remove: /tmp/example/single_git/modules/firewall/manifests/stale.pp called from checkForStaleContent()",
		"DEBUG purgeDir(): Trying to remove: /tmp/example/single_git/stale_directory_that_should_be_purged called from checkForStaleContent()",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	expectedFiles := []string{
		"/tmp/example/single_git/.resource_types",
		"/tmp/example/single_git/.resource_types/foobar.pp",
		"/tmp/example/single_git/.latest_revision",
	}

	for _, expectedFile := range expectedFiles {
		if !fileExists(expectedFile) {
			t.Errorf("purge_whitelist item was purged: " + expectedFile)
		}
	}

	missingFiles := []string{
		"/tmp/example/single_git/stale_directory_that_should_be_purged",
		"/tmp/example/single_git/modules/firewall/manifests/stale.pp",
	}
	for _, missingFile := range missingFiles {
		if fileExists(missingFile) {
			t.Errorf("stale file and/or directory still exists! " + missingFile)
		}
	}

	if !fileExists("/tmp/example/single_git/modules/firewall/README.markdown") {
		t.Errorf("Missing module file that should be there")
	}

	purgeDir(cacheDir, funcName)
	purgeDir("/tmp/example", funcName)
}

func TestPurgeWhitelistRecursive(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	cacheDir := "/tmp/g10k"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		config = readConfigfile("tests/TestConfigExamplePurgeEnvironmentRecursive.yaml")
		branchParam = "single_git"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	createOrPurgeDir("/tmp/example/single_git/stale_directory_that_should_be_purged", funcName)
	createOrPurgeDir("/tmp/example/single_git/.resource_types", funcName)
	f, _ := os.Create("/tmp/example/single_git/.latest_revision")
	defer f.Close()
	f.WriteString("foobar")
	f.Sync()
	frt, _ := os.Create("/tmp/example/single_git/.resource_types/foobar.pp")
	defer frt.Close()
	frt.WriteString("fake resource type")
	frt.Sync()
	createOrPurgeDir("/tmp/example/single_git/modules/", funcName)
	createOrPurgeDir("/tmp/example/single_git/modules/firewall/", funcName)
	createOrPurgeDir("/tmp/example/single_git/modules/firewall/manifests/", funcName)
	fpf, _ := os.Create("/tmp/example/single_git/modules/firewall/manifests/stale.pp")
	defer fpf.Close()
	fpf.WriteString("fake stale module file")
	fpf.Sync()

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"Removing unmanaged path /tmp/example/single_git/stale_directory_that_should_be_purged",
		"DEBUG purgeDir(): Trying to remove: /tmp/example/single_git/stale_directory_that_should_be_purged called from checkForStaleContent()",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	expectedFiles := []string{
		"/tmp/example/single_git/.resource_types",
		"/tmp/example/single_git/.resource_types/foobar.pp",
		"/tmp/example/single_git/.latest_revision",
		"/tmp/example/single_git/modules/firewall/manifests/stale.pp",
	}

	for _, expectedFile := range expectedFiles {
		if !fileExists(expectedFile) {
			t.Errorf("purge_whitelist item was purged: " + expectedFile)
		}
	}

	missingFiles := []string{
		"/tmp/example/single_git/stale_directory_that_should_be_purged",
	}
	for _, missingFile := range missingFiles {
		if fileExists(missingFile) {
			t.Errorf("stale file and/or directory still exists! " + missingFile)
		}
	}

	if !fileExists("/tmp/example/single_git/modules/firewall/README.markdown") {
		t.Errorf("Missing module file that should be there")
	}

	purgeDir(cacheDir, funcName)
	purgeDir("/tmp/example", funcName)
}

func TestPurgeStaleContent(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	cacheDir := "/tmp/g10k"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		config = readConfigfile("tests/TestConfigExamplePurgeEnvironment.yaml")
		branchParam = "single"
		resolvePuppetEnvironment(false, "")
		return
	}
	createOrPurgeDir("/tmp/example/single/stale_directory_that_should_be_purged", funcName)
	createOrPurgeDir("/tmp/example/single/stale_directory_that_should_be_purged2", funcName)
	f, _ := os.Create("/tmp/example/single/stale_directory_that_should_be_purged/stale_file")
	defer f.Close()
	f.WriteString("foobar")
	f.Sync()

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"DEBUG checkForStaleContent(): filepath.Walk'ing directory /tmp/example/single",
		"Removing unmanaged path /tmp/example/single/stale_directory_that_should_be_purged",
		"DEBUG purgeDir(): Trying to remove: /tmp/example/single/stale_directory_that_should_be_purged called from checkForStaleContent()",
		"Removing unmanaged path /tmp/example/single/stale_directory_that_should_be_purged/stale_file",
		"DEBUG purgeDir(): Unnecessary to remove dir: /tmp/example/single/stale_directory_that_should_be_purged/stale_file it does not exist. Called from checkForStaleContent()",
		"Removing unmanaged path /tmp/example/single/stale_directory_that_should_be_purged2",
		"DEBUG purgeDir(): Trying to remove: /tmp/example/single/stale_directory_that_should_be_purged2 called from checkForStaleContent()",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	if fileExists("/tmp/example/single/stale_directory_that_should_be_purged/stale_file") ||
		fileExists("/tmp/example/single/stale_directory_that_should_be_purged") ||
		fileExists("/tmp/example/single/stale_directory_that_should_be_purged2") {
		t.Errorf("stale file and/or directory still exists!")
	}

	if !fileExists("/tmp/example/single/external_modules/inifile/README.md") {
		t.Errorf("Missing module file that should be there")
	}

	purgeDir(cacheDir, funcName)
	purgeDir("/tmp/example", funcName)
}

func TestPurgeStaleEnvironments(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	cacheDir := "/tmp/g10k"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		config = readConfigfile("tests/TestConfigFullworking.yaml")
		resolvePuppetEnvironment(false, "")
		return
	}
	createOrPurgeDir("/tmp/full/full_stale/stale_directory_that_should_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_stale/stale_dir", funcName)
	f, _ := os.Create("/tmp/full/full_stale/stale_dir/stale_file")
	defer f.Close()
	f.WriteString("foobar")
	f.Sync()

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"DEBUG purgeUnmanagedContent(): Glob'ing with path /tmp/full/full_*",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_another",
		"DEBUG purgeUnmanagedContent(): Not purging environment full_another",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_master",
		"DEBUG purgeUnmanagedContent(): Not purging environment full_master",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_qa",
		"DEBUG purgeUnmanagedContent(): Not purging environment full_qa",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_stale",
		"Removing unmanaged environment full_stale",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	if fileExists("/tmp/full/full_stale/stale_directory_that_should_be_purged") ||
		fileExists("/tmp/full/full_stale/stale_dir") ||
		fileExists("/tmp/full/full_stale/stale_dir/stale_file") {
		t.Errorf("stale file and/or directory still exists!")
	}

	if !fileExists("/tmp/full/full_master/modules/stdlib/metadata.json") {
		t.Errorf("Missing module file that should be there")
	}

	purgeDir(cacheDir, funcName)
	purgeDir("/tmp/full", funcName)
}

func TestPurgeStaleEnvironmentOnly(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	cacheDir := "/tmp/g10k"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		config = readConfigfile("tests/TestConfigFullworkingPurgeEnvironment.yaml")
		fmt.Printf("%+v\n", config)
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}
	createOrPurgeDir("/tmp/full/full_master/modules/stale_module_directory_that_should_not_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_master/stale_directory_that_should_not_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_stale/stale_directory_that_should_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_stale/stale_dir", funcName)
	f, _ := os.Create("/tmp/full/full_stale/stale_dir/stale_file")
	defer f.Close()
	f.WriteString("foobar")
	f.Sync()

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"Removing unmanaged path /tmp/full/full_master/stale_directory_that_should_not_be_purged",
		"DEBUG purgeDir(): Trying to remove: /tmp/full/full_master/stale_directory_that_should_not_be_purged called from checkForStaleContent()",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	missingFiles := []string{
		"/tmp/full/full_master/stale_directory_that_should_not_be_purged",
	}
	for _, missingFile := range missingFiles {
		if fileExists(missingFile) {
			t.Errorf("stale file and/or directory still exists! " + missingFile)
		}
	}

	expectedFiles := []string{
		"/tmp/full/full_stale/stale_directory_that_should_be_purged",
		"/tmp/full/full_stale/stale_dir",
		"/tmp/full/full_stale/stale_dir/stale_file",
	}
	for _, expectedFile := range expectedFiles {
		if !fileExists(expectedFile) {
			t.Errorf("stale files and/or directory missing that should not have been purged! " + expectedFile)
		}
	}

	if !fileExists("/tmp/full/full_master/modules/stdlib/metadata.json") {
		t.Errorf("Missing module file that should be there")
	}

	purgeDir(cacheDir, funcName)
	purgeDir("/tmp/full", funcName)
}

func TestPurgeStalePuppetfileOnly(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	cacheDir := "/tmp/g10k"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		config = readConfigfile("tests/TestConfigFullworkingPurgePuppetfile.yaml")
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}
	createOrPurgeDir("/tmp/full/full_master/modules/stale_module_directory_that_should_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_master/stale_directory_that_should_not_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_stale/stale_directory_that_should_not_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_stale/stale_dir", funcName)
	f, _ := os.Create("/tmp/full/full_stale/stale_dir/stale_file")
	defer f.Close()
	f.WriteString("foobar")
	f.Sync()

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"Removing unmanaged path /tmp/full/full_master/modules/stale_module_directory_that_should_be_purged",
		"DEBUG purgeDir(): Trying to remove: /tmp/full/full_master/modules/stale_module_directory_that_should_be_purged called from purge_level puppetfile",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	missingFiles := []string{
		"/tmp/full/full_master/modules/stale_module_directory_that_should_be_purged",
	}
	for _, missingFile := range missingFiles {
		if fileExists(missingFile) {
			t.Errorf("stale file and/or directory still exists! " + missingFile)
		}
	}

	expectedFiles := []string{
		"/tmp/full/full_master/stale_directory_that_should_not_be_purged",
		"/tmp/full/full_stale/stale_directory_that_should_not_be_purged",
		"/tmp/full/full_stale/stale_dir",
		"/tmp/full/full_stale/stale_dir/stale_file",
	}
	for _, expectedFile := range expectedFiles {
		if !fileExists(expectedFile) {
			t.Errorf("stale files and/or directory missing that should not have been purged! " + expectedFile)
		}
	}

	if !fileExists("/tmp/full/full_master/modules/stdlib/metadata.json") {
		t.Errorf("Missing module file that should be there")
	}

	purgeDir(cacheDir, funcName)
	purgeDir("/tmp/full", funcName)
}

func TestPurgeStaleDeploymentOnly(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	cacheDir := "/tmp/g10k"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		config = readConfigfile("tests/TestConfigFullworkingPurgeDeployment.yaml")
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}
	createOrPurgeDir("/tmp/full/full_master/modules/stale_module_directory_that_should_not_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_master/stale_directory_that_should_not_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_stale/stale_directory_that_should_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_stale/stale_dir", funcName)
	f, _ := os.Create("/tmp/full/full_stale/stale_dir/stale_file")
	defer f.Close()
	f.WriteString("foobar")
	f.Sync()

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"DEBUG purgeUnmanagedContent(): Glob'ing with path /tmp/full/full_*",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_another",
		"DEBUG purgeUnmanagedContent(): Not purging environment full_another",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_master",
		"DEBUG purgeUnmanagedContent(): Not purging environment full_master",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_qa",
		"DEBUG purgeUnmanagedContent(): Not purging environment full_qa",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_stale",
		"Removing unmanaged environment full_stale",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	if fileExists("/tmp/full/full_stale/stale_directory_that_should_be_purged") ||
		fileExists("/tmp/full/full_stale/stale_dir") ||
		fileExists("/tmp/full/full_stale/stale_dir/stale_file") {
		t.Errorf("stale file and/or directory still exists!")
	}
	if !fileExists("/tmp/full/full_master/modules/stale_module_directory_that_should_not_be_purged") ||
		!fileExists("/tmp/full/full_master/stale_directory_that_should_not_be_purged") {
		t.Errorf("stale files and/or directory missing that should not have been purged!")
	}

	if !fileExists("/tmp/full/full_master/modules/stdlib/metadata.json") {
		t.Errorf("Missing module file that should be there")
	}

	purgeDir(cacheDir, funcName)
	purgeDir("/tmp/full", funcName)
}

func TestPurgeStaleDeploymentOnlyWithWhitelist(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	cacheDir := "/tmp/g10k"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		config = readConfigfile("tests/TestConfigFullworkingPurgeDeploymentWithWhitelist.yaml")
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}
	createOrPurgeDir("/tmp/full/full_master/modules/stale_module_directory_that_should_not_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_master/stale_directory_that_should_not_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_stale/stale_directory_that_should_be_purged", funcName)
	createOrPurgeDir("/tmp/full/full_stale/stale_dir", funcName)
	createOrPurgeDir("/tmp/full/full_hiera_master/hiera_dir", funcName)
	createOrPurgeDir("/tmp/full/full_hiera_qa/hiera_dir_qa", funcName)
	f, _ := os.Create("/tmp/full/full_stale/stale_dir/stale_file")
	defer f.Close()
	f.WriteString("foobar")
	f.Sync()

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"DEBUG purgeUnmanagedContent(): Glob'ing with path /tmp/full/full_*",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_another",
		"DEBUG purgeUnmanagedContent(): Not purging environment full_another",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_master",
		"DEBUG purgeUnmanagedContent(): Not purging environment full_master",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_qa",
		"DEBUG purgeUnmanagedContent(): Not purging environment full_qa",
		"DEBUG purgeUnmanagedContent(): Checking if environment should exist: full_stale",
		"Removing unmanaged environment full_stale",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	if fileExists("/tmp/full/full_stale/stale_directory_that_should_be_purged") ||
		fileExists("/tmp/full/full_stale/stale_dir") ||
		fileExists("/tmp/full/full_stale/stale_dir/stale_file") {
		t.Errorf("stale file and/or directory still exists!")
	}

	expectedFiles := []string{
		"/tmp/full/full_master/modules/stale_module_directory_that_should_not_be_purged",
		"/tmp/full/full_master/stale_directory_that_should_not_be_purged",
		"/tmp/full/full_hiera_qa/hiera_dir_qa",
		"/tmp/full/full_hiera_master/hiera_dir"}

	for _, expectedFile := range expectedFiles {
		if !fileExists(expectedFile) {
			t.Errorf("stale files and/or directory missing that should not have been purged! " + expectedFile)
		}
	}

	if !fileExists("/tmp/full/full_master/modules/stdlib/metadata.json") {
		t.Errorf("Missing module file that should be there")
	}

	purgeDir(cacheDir, funcName)
	purgeDir("/tmp/full", funcName)
}

func TestEnvironmentParameter(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	cacheDir := "/tmp/g10k"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		config = readConfigfile("tests/TestConfigFullworkingAndExampleDifferentPrefix.yaml")
		environmentParam = "full_master"
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"DEBUG 1(): Resolving environment master of source full",
		"DEBUG resolvePuppetfile(): Resolving branch master of source full",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	if fileExists("/tmp/out/example_master") {
		t.Errorf("Puppet environment example_master should not have been deployed, with branch parameter set to full_master")
	}

	expectedFiles := []string{
		"/tmp/out/master/modules/stdlib/metadata.json",
	}

	for _, expectedFile := range expectedFiles {
		if !fileExists(expectedFile) {
			t.Errorf("Puppet environment full_master seems not to have been populated " + expectedFile)
		}
	}

	purgeDir(cacheDir, funcName)
	purgeDir("/tmp/out", funcName)
}

func TestSkipPurgingWithMultipleSources(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/both.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		environmentParam = "example_single"
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		environmentParam = "full_single"
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		environmentParam = "example_single_git"
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		// create stale sub folder with a file inside
		checkDirAndCreate("/tmp/out/example_single_git/mymodule2/dir1", funcName)
		f, _ := os.Create("/tmp/out/example_single_git/mymodule2/dir1/file3")
		f.WriteString("slddkasjld")
		f.Close()
		// do not manipulate a module locally, because r10k and g10k doesn't scan
		// the module's directory content, if its version/commit hash didn't change
		//f, _ = os.Create("/tmp/out/example_single_git/modules/firewall/unmanaged_file")
		//f.WriteString("slddkasjld33")
		//f.Close()
		f.Sync()
		branchParam = ""
		resolvePuppetEnvironment(false, "")

		return
	}

	purgeDir("/tmp/out", funcName)

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"Need to sync /tmp/out/example_single",
		"Need to sync /tmp/out/full_single",
		"DEBUG purgeUnmanagedContent(): Skipping purging unmanaged content for source 'example', because -environment parameter is set to full_single",
		"Need to sync /tmp/out/example_single_git",
		"DEBUG purgeUnmanagedContent(): Skipping purging unmanaged content for Puppet environment 'example_single', because -environment parameter is set to example_single_git",
		"DEBUG purgeUnmanagedContent(): Skipping purging unmanaged content for source 'full', because -environment parameter is set to example_single_git",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	path, err := exec.LookPath("hashdeep")
	if err != nil {
		t.Skip("Skipping full Puppet environment resolve test, because package hashdeep is missing")
	}

	// remove timestamps from .g10k-deploy.json otherwise hash sum would always differ
	removeTimestampsFromDeployfile("/tmp/out/example_single/.g10k-deploy.json")
	removeTimestampsFromDeployfile("/tmp/out/full_single/.g10k-deploy.json")
	removeTimestampsFromDeployfile("/tmp/out/example_single_git/.g10k-deploy.json")

	cmd = exec.Command(path, "-vv", "-l", "-r", "-a", "-k", "tests/hashdeep_both_multiple.hashdeep", "/tmp/out")
	out, err = cmd.CombinedOutput()
	exitCode = 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}
	if exitCode != 0 {
		t.Errorf("hashdeep terminated with %v, but we expected exit status 0\nOutput: %v", exitCode, string(out))
	}

}

func TestSymlink(t *testing.T) {
	path, err := exec.LookPath("hashdeep")
	if err != nil {
		t.Skip("Skipping full Puppet environment resolve test, because package hashdeep is missing")
	}

	quiet = true
	purgeDir("/tmp/g10k", "TestSymlink()")
	purgeDir("/tmp/out", "TestSymlink()")
	config = readConfigfile("tests/both.yaml")
	// increase maxworker to finish the test quicker
	config.Maxworker = 500
	environmentParam = "full_symlinks"

	//do it twice to detect errors
	for i := 0; i < 3; {
		i++

		resolvePuppetEnvironment(false, "")

		// remove timestamps from .g10k-deploy.json otherwise hash sum would always differ
		removeTimestampsFromDeployfile("/tmp/out/full_symlinks/.g10k-deploy.json")

		cmd := exec.Command(path, "-vv", "-l", "-r", "-a", "-k", "tests/hashdeep_both_symlinks.hashdeep", "/tmp/out")
		out, err := cmd.CombinedOutput()
		exitCode := 0
		if msg, ok := err.(*exec.ExitError); ok { // there is error code
			exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
		}
		if exitCode != 0 {
			t.Errorf("hashdeep terminated with %v, but we expected exit status 0\nOutput: %v", exitCode, string(out))
		}
		if !strings.Contains(string(out), "") {
			t.Errorf("resolvePuppetfile() terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
		}
		Debugf("hashdeep output:" + string(out))

		// check if the symlinks with non-existent targets are there #150
		// because hashdeep ignores them
		invalidSymlinks := []string{
			"/tmp/out/full_symlinks/modules/testmodule/not-working-symlink",
			"/tmp/out/full_symlinks/1/not-working-symlink",
		}

		for _, invalidSymlink := range invalidSymlinks {
			if !fileExists(invalidSymlink) {
				t.Errorf("symlink with non-existent target missing: %s", invalidSymlink)
			}
		}

		purgeDir("/tmp/out/full_symlinks/modules/testmodule/files/docs/another_dir/file", "TestResolveStatic()")

		cmd = exec.Command("hashdeep", "-l", "-r", "-a", "-k", "tests/hashdeep_both_symlinks.hashdeep", "/tmp/out")
		out, err = cmd.CombinedOutput()
		exitCode = 0
		if msg, ok := err.(*exec.ExitError); ok { // there is error code
			exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
		}

		if exitCode != 1 {
			t.Errorf("hashdeep terminated with %v, but we expected exit status 1\nOutput: %v", exitCode, string(out))
		}
		purgeDir("/tmp/out/full_symlinks/modules/testmodule/.latest_commit", "TestResolveStatic()")

		f, _ := os.Create("/tmp/out/full_symlinks/modules/testmodule/.latest_commit")
		defer f.Close()
		f.WriteString("foobarinvalidgitcommithashthatshouldtriggeraresyncofthismodule")
		f.Sync()

	}
	environmentParam = ""
}

func TestAutoCorrectEnvironmentNamesPurge(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/autocorrect.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = false
		info = true
		environmentParam = ""
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}

	purgeDir("/tmp/out", funcName)

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"Renaming branch invalid-name to invalid_name",
		"Need to sync /tmp/out/autocorrect_invalid_name",
		"Need to sync /tmp/out/autocorrect_master",
		"Need to sync /tmp/out/autocorrect_invalid_name/modules/inifile",
		"Need to sync /tmp/out/autocorrect_master/modules/inifile",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	expectedFiles := []string{
		"/tmp/out/autocorrect_master/",
		"/tmp/out/autocorrect_master/Puppetfile",
		"/tmp/out/autocorrect_master/modules/inifile/metadata.json",
		"/tmp/out/autocorrect_invalid_name/",
		"/tmp/out/autocorrect_invalid_name/Puppetfile",
		"/tmp/out/autocorrect_invalid_name/modules/inifile/metadata.json",
	}

	for _, expectedFile := range expectedFiles {
		if !fileExists(expectedFile) {
			t.Errorf("Puppet environment/module file missing: " + expectedFile)
		}
	}

	purgeDir("/tmp/out", funcName)

}

func TestUnresolveableModuleReferenceOutputGit(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/failingEnvGit.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = false
		info = true
		environmentParam = ""
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}

	purgeDir("/tmp/failgit", funcName)

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 1
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"Failed to resolve git module 'testmodule' with repository https://github.com/xorpaul/g10k_testmodule.git and branch/reference 'nonexisting' used in control repository branch 'failing_branch_git' or Puppet environment 'failgit_failing_branch_git'",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in output")
		}
	}
}

func TestUnresolveableModuleReferenceOutputForge(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/failingEnvForge.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = false
		info = true
		environmentParam = ""
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}

	purgeDir("/tmp/failforge", funcName)

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 1
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"Received 404 from Forge using URL https://forgeapi.puppet.com/v3/files/puppetlabs-stdlib-0.0.1.tar.gz",
		"Check if the module name 'puppetlabs-stdlib' and version '0.0.1' really exist",
		"Used in Puppet environment 'failforge_failing_branch_forge'",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in output")
		}
	}
}

func TestCloneGitModules(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigFullworkingCloneGitModules.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		environmentParam = ""
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}

	purgeDir("/tmp/full", funcName)

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))

	expectedLines := []string{
		"DEBUG executeCommand(): Executing git clone --single-branch --branch 11.0.0 https://github.com/theforeman/puppet-puppet.git /tmp/full/full_master/modules/puppet",
		"DEBUG executeCommand(): Executing git clone --single-branch --branch release https://github.com/puppetlabs/puppetlabs-stdlib.git /tmp/full/full_another/modules/stdlib",
		"DEBUG executeCommand(): Executing git clone --single-branch --branch symlinks https://github.com/xorpaul/g10k-test-module.git /tmp/full/full_symlinks/modules/testmodule",
		"DEBUG executeCommand(): Executing git clone --single-branch --branch master https://github.com/elastic/puppet-kibana.git /tmp/full/full_qa/modules/kibana",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in output")
		}
	}

	expectedDirs := []string{
		"/tmp/full/full_master/modules/puppet/.git",
		"/tmp/full/full_another/modules/stdlib/.git",
		"/tmp/full/full_symlinks/modules/testmodule/.git",
		"/tmp/full/full_qa/modules/kibana/.git",
	}

	for _, expectedDir := range expectedDirs {
		if !isDir(expectedDir) {
			t.Errorf("This Puppet module is not a cloned git repository despite clone_git_modules set to true :" + expectedDir)
		}
	}
	// check
	for _, expectedDir := range expectedDirs {
		headFile := filepath.Join(expectedDir, "HEAD")
		content, err := ioutil.ReadFile(headFile)
		if err != nil {
			t.Errorf("Error while reading content of file " + headFile + " Error: " + err.Error())
		}
		stringContent := string(content)
		if headFile == "/tmp/full/full_another/modules/stdlib/.git/HEAD" {
			expectedBranch := "ref: refs/heads/release"
			if strings.TrimRight(stringContent, "\n") != expectedBranch {
				t.Errorf("Error wrong branch found in checked out Git repo for " + expectedDir + " We expected " + expectedBranch + ", but found content: " + stringContent)
			}
		}
	}
}

func TestPrivateGithubRepository(t *testing.T) {
	path := "tests/github-test-private/github-test-private"
	if !fileExists(path) {
		t.Skip("Skipping TestPrivateGithubRepository test, because the test SSH key '" + path + "' is missing")
	}
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrivateGithub.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		environmentParam = ""
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}

	purgeDir("/tmp/private", funcName)

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	_, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))
	expectedFiles := []string{
		"/tmp/private/master/Puppetfile",
		"/tmp/private/master/modules/testmodule/manifests/init.pp",
	}

	for _, expectedFile := range expectedFiles {
		if !fileExists(expectedFile) {
			t.Errorf("Puppet environment/module file missing: " + expectedFile)
		}
	}

	purgeDir("/tmp/private", funcName)
}

func TestBranchFilterCommand(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigFullworkingBranchFilter.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		environmentParam = ""
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}

	purgeDir("/tmp/branchfilter", funcName)

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	_, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	//fmt.Println(string(out))
	expectedFiles := []string{
		"/tmp/branchfilter/full_single/.g10k-deploy.json",
		"/tmp/branchfilter/full_single/Puppetfile",
		"/tmp/branchfilter/full_single/modules/ntp/manifests/init.pp",
		"/tmp/branchfilter/full_master/.g10k-deploy.json",
		"/tmp/branchfilter/full_master/Puppetfile",
		"/tmp/branchfilter/full_master/modules/stdlib/manifests/init.pp",
		"/tmp/branchfilter/full_master/modules/puppet/manifests/init.pp",
	}

	for _, expectedFile := range expectedFiles {
		if !fileExists(expectedFile) {
			t.Errorf("Puppet environment/module file missing: " + expectedFile)
		}
	}

	expectedMissingFiles := []string{
		"/tmp/branchfilter/full_qa/.g10k-deploy.json",
		"/tmp/branchfilter/full_qa/modules/apache/manifests/init.pp",
		"/tmp/branchfilter/full_symlinks/Puppetfile",
	}
	for _, expectedMissingFile := range expectedMissingFiles {
		if fileExists(expectedMissingFile) {
			t.Errorf("Found Puppet environment files, which should've been filtered out by filter_command" + expectedMissingFile)
		}
	}

	purgeDir("/tmp/branchfilter", funcName)
}

func TestBranchFilterRegex(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigFullworkingBranchFilterRegex.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		environmentParam = ""
		branchParam = ""
		resolvePuppetEnvironment(false, "")
		return
	}

	purgeDir("/tmp/branchfilter", funcName)

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	_, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	expectedExitCode := 0
	if expectedExitCode != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, expectedExitCode)
	}
	expectedFiles := []string{
		"/tmp/branchfilter/full_single/.g10k-deploy.json",
		"/tmp/branchfilter/full_single/Puppetfile",
		"/tmp/branchfilter/full_single/modules/ntp/manifests/init.pp",
		"/tmp/branchfilter/full_master/.g10k-deploy.json",
		"/tmp/branchfilter/full_master/Puppetfile",
		"/tmp/branchfilter/full_master/modules/stdlib/manifests/init.pp",
		"/tmp/branchfilter/full_master/modules/puppet/manifests/init.pp",
	}

	for _, expectedFile := range expectedFiles {
		if !fileExists(expectedFile) {
			t.Errorf("Puppet environment/module file missing: " + expectedFile)
		}
	}

	expectedMissingFiles := []string{
		"/tmp/branchfilter/full_qa/.g10k-deploy.json",
		"/tmp/branchfilter/full_qa/modules/apache/manifests/init.pp",
		"/tmp/branchfilter/full_symlinks/Puppetfile",
	}
	for _, expectedMissingFile := range expectedMissingFiles {
		if fileExists(expectedMissingFile) {
			t.Errorf("Found Puppet environment files, which should've been filtered out by filter_command" + expectedMissingFile)
		}
	}

	purgeDir("/tmp/branchfilter", funcName)
}

func TestResolvePuppetfileUseSSHAgent(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	configFile = "tests/TestConfigUseSSHAgent.yaml"
	config = readConfigfile(configFile)
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		purgeDir("/tmp/example/", funcName)
		purgeDir("/tmp/g10k/", funcName)
		debug = true
		branchParam = "use_ssh_agent"
		resolvePuppetEnvironment(false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 1 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 1, string(out))
	}
	//fmt.Println(string(out))

	sshAddCmd := "ssh-add"
	if runtime.GOOS == "darwin" {
		sshAddCmd = "ssh-add -K"
	}

	expectedLines := []string{
		"DEBUG git repo url git@local.git.server:foo/git_module_with_ssh_agent.git with loaded SSH keys from ssh-agent",
		"DEBUG git repo url git@github.com:foobar/github_module_without_ssh_add.git with SSH key tests/TestConfigUseSSHAgent.yaml",
		"DEBUG git repo url git@local.git.server:bar/git_module_with_ssh_add.git with SSH key tests/TestConfigUseSSHAgent.yaml",
		"DEBUG executeCommand(): Executing git clone --mirror git@local.git.server:foo/git_module_with_ssh_agent.git /tmp/g10k/modules/git@local.git.server-foo_git_module_with_ssh_agent.git",
		"DEBUG executeCommand(): Executing git clone --mirror git@github.com:foobar/github_module_without_ssh_add.git /tmp/g10k/modules/git@github.com-foobar_github_module_without_ssh_add.git",
		"DEBUG executeCommand(): Executing ssh-agent bash -c '" + sshAddCmd + " tests/TestConfigUseSSHAgent.yaml; git clone --mirror git@local.git.server:bar/git_module_with_ssh_add.git /tmp/g10k/modules/git@local.git.server-bar_git_module_with_ssh_add.git'",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	moduleParam = ""
	debug = false

}

func TestResolvePuppetfileAutoDetectDefaultBranch(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigUseCacheFallback.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		//debug = true
		branchParam = "single_git_non_master_as_default"
		resolvePuppetEnvironment(false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	branchParam = "single_git_non_master_as_default"
	resolvePuppetEnvironment(false, "")

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 0 {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))
}
