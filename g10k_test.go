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
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
)

func TestForgeChecksum(t *testing.T) {
	expectedFmm := ForgeModule{md5sum: "8a8c741978e578921e489774f05e9a65", fileSize: 57358}
	fmm := getMetadataForgeModule(ForgeModule{version: "2.2.0", name: "apt",
		author: "puppetlabs", baseURL: "https://forgeapi.puppetlabs.com"})

	if fmm.md5sum != expectedFmm.md5sum {
		t.Error("Expected md5sum", expectedFmm.md5sum, "got", fmm.md5sum)
	}

	if fmm.fileSize != expectedFmm.fileSize {
		t.Error("Expected fileSize", expectedFmm.fileSize, "got", fmm.fileSize)
	}
}

func TestConfigPrefix(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile("tests/" + funcName + ".yaml")

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-environment.git",
		Basedir: "/tmp/example/", Prefix: "foobar", PrivateKey: ""}

	expected := ConfigSettings{
		CacheDir: "/tmp/g10k/", ForgeCacheDir: "/tmp/g10k/forge/",
		ModulesCacheDir: "/tmp/g10k/modules/", EnvCacheDir: "/tmp/g10k/environments/",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppetlabs.com"},
		Sources: s, Timeout: 5, Maxworker: 50, MaxExtractworker: 20,
		PurgeLevels: []string{"deployment", "puppetfile"}}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Expected ConfigSettings: %+v, but got ConfigSettings: %+v", expected, got)
	}
}

func TestConfigForceForgeVersions(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile("tests/" + funcName + ".yaml")

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-environment.git",
		Basedir: "/tmp/example/", Prefix: "foobar", PrivateKey: "", ForceForgeVersions: true, WarnMissingBranch: false}

	expected := ConfigSettings{
		CacheDir: "/tmp/g10k/", ForgeCacheDir: "/tmp/g10k/forge/",
		ModulesCacheDir: "/tmp/g10k/modules/", EnvCacheDir: "/tmp/g10k/environments/",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppetlabs.com"},
		Sources: s, Timeout: 5, Maxworker: 50, MaxExtractworker: 20,
		PurgeLevels: []string{"deployment", "puppetfile"}}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Expected ConfigSettings: %+v, but got ConfigSettings: %+v", expected, got)
	}
}

func TestConfigAddWarning(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile("tests/" + funcName + ".yaml")

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-environment.git",
		Basedir: "/tmp/example/", PrivateKey: "", ForceForgeVersions: false, WarnMissingBranch: true}

	expected := ConfigSettings{
		CacheDir: "/tmp/g10k/", ForgeCacheDir: "/tmp/g10k/forge/",
		ModulesCacheDir: "/tmp/g10k/modules/", EnvCacheDir: "/tmp/g10k/environments/",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppetlabs.com"},
		Sources: s, Timeout: 5, Maxworker: 50, MaxExtractworker: 20,
		PurgeLevels: []string{"deployment", "puppetfile"}}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Expected ConfigSettings: %+v, but got ConfigSettings: %+v", expected, got)
	}
}

func TestConfigSimplePostrunCommand(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile("tests/" + funcName + ".yaml")

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-environment.git",
		Basedir: "/tmp/example/", PrivateKey: "", ForceForgeVersions: false}

	postrunCommand := []string{"/usr/bin/touch", "-f", "/tmp/g10kfoobar"}
	expected := ConfigSettings{
		CacheDir: "/tmp/g10k/", ForgeCacheDir: "/tmp/g10k/forge/",
		ModulesCacheDir: "/tmp/g10k/modules/", EnvCacheDir: "/tmp/g10k/environments/",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppetlabs.com"},
		Sources: s, Timeout: 5, Maxworker: 50, MaxExtractworker: 20,
		PurgeLevels: []string{"deployment", "puppetfile"}, PostRunCommand: postrunCommand}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Expected ConfigSettings: %+v, but got ConfigSettings: %+v", expected, got)
	}
}

func TestConfigPostrunCommand(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile("tests/" + funcName + ".yaml")

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-test-environment.git",
		Basedir: "/tmp/example/", PrivateKey: "", ForceForgeVersions: false, Prefix: "true"}

	postrunCommand := []string{"tests/postrun.sh", "$modifiedenvs"}
	expected := ConfigSettings{
		CacheDir: "/tmp/g10k/", ForgeCacheDir: "/tmp/g10k/forge/",
		ModulesCacheDir: "/tmp/g10k/modules/", EnvCacheDir: "/tmp/g10k/environments/",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppetlabs.com"},
		Sources: s, Timeout: 5, Maxworker: 50, MaxExtractworker: 20,
		PurgeLevels: []string{"deployment", "puppetfile"}, PostRunCommand: postrunCommand}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Expected ConfigSettings: %+v, but got ConfigSettings: %+v", expected, got)
	}
}

func TestConfigDeploy(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readConfigfile("tests/" + funcName + ".yaml")

	s := make(map[string]Source)
	s["full"] = Source{Remote: "https://github.com/xorpaul/g10k-fullworking-env.git",
		Basedir: "/tmp/full/", Prefix: "true", PrivateKey: ""}

	expected := ConfigSettings{
		CacheDir: "/tmp/g10k/", ForgeCacheDir: "/tmp/g10k/forge/",
		ModulesCacheDir: "/tmp/g10k/modules/", EnvCacheDir: "/tmp/g10k/environments/",
		Git:     Git{privateKey: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppetlabs.com"},
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

func TestResolvConfigAddWarning(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigAddWarning.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		resolvePuppetEnvironment("nonExistingBranch", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	if !strings.Contains(string(out), "WARNING: Couldn't find specified branch 'nonExistingBranch' anywhere in source 'example' (https://github.com/xorpaul/g10k-environment.git)") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
}

func TestResolvStatic(t *testing.T) {

	path, err := exec.LookPath("hashdeep")
	if err != nil {
		t.Skip("Skipping full Puppet environment resolv test, because package hashdeep is missing")
	}

	quiet = true
	purgeDir("./cache/", "TestResolvStatic()")
	purgeDir("./example/", "TestResolvStatic()")
	config = readConfigfile("tests/TestConfigStatic.yaml")
	// increase maxworker to finish the test quicker
	config.Maxworker = 500
	resolvePuppetEnvironment("static", false, "")

	cmd := exec.Command(path, "-vvv", "-l", "-r", "./example", "-a", "-k", "tests/hashdeep_example_static.hashdeep")
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

	purgeDir("example/example_static/external_modules/stdlib/spec/unit/facter/util", "TestResolvStatic()")

	cmd = exec.Command("hashdeep", "-r", "./example/", "-a", "-k", "tests/hashdeep_example_static.hashdeep")
	out, err = cmd.CombinedOutput()
	exitCode = 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if exitCode != 1 {
		t.Errorf("hashdeep terminated with %v, but we expected exit status 1\nOutput: %v", exitCode, string(out))
	}

	fileMode, err := os.Stat("./example/example_static/external_modules/aws/examples/audit-security-groups/count_out_of_sync_resources.sh")
	if fileMode.Mode().String() != "-rwxrwxr-x" {
		t.Error("Wrong file permission for test file. Check unTar()")
	}

}

func TestConfigGlobalAllowFail(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/" + funcName + ".yaml")

	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		resolvePuppetEnvironment("", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	if !strings.Contains(string(out), "Failed to populate module /tmp/failing/master/modules/sensu/ but ignore-unreachable is set. Continuing...") {
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
		forgeBaseURL: f.baseURL, workDir: "/tmp/test_test/"}
	pfm := make(map[string]Puppetfile)
	pfm["test"] = pf

	config = ConfigSettings{ForgeCacheDir: "/tmp/forge_cache", Maxworker: 500}
	defer purgeDir(pf.workDir, "TestInvalidMetadataForgemodule")
	defer purgeDir(config.ForgeCacheDir, "TestInvalidMetadataForgemodule")

	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
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
	if !strings.Contains(string(out), "WARNING: calculated file size 760 for /tmp/forge_cachepuppetlabs-ntp-6.0.0.tar.gz does not match expected file size 1337") {
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
		forgeBaseURL: f.baseURL, workDir: "/tmp/test_test/"}
	pfm := make(map[string]Puppetfile)
	pfm["test"] = pf

	config = ConfigSettings{ForgeCacheDir: "/tmp/forge_cache", Maxworker: 500}
	defer purgeDir(pf.workDir, "TestInvalidMetadataForgemodule")
	defer purgeDir(config.ForgeCacheDir, "TestInvalidMetadataForgemodule")

	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		resolvePuppetfile(pfm)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() && strings.Contains(string(out), "WARNING: calculated md5sum ccee7dd0c564de1c586be58dcf7626a5 for /tmp/forge_cachepuppetlabs-ntp-6.0.0.tar.gz does not match expected md5sum fakeMd5SumToCheckIfIntegrityCheckWorksAsExpected") {
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
		forgeBaseURL: f.baseURL, workDir: "/tmp/test_test/"}
	pfm := make(map[string]Puppetfile)
	pfm["test"] = pf

	config = ConfigSettings{ForgeCacheDir: "/tmp/forge_cache", Maxworker: 500}
	defer purgeDir(pf.workDir, "TestInvalidMetadataForgemodule")
	defer purgeDir(config.ForgeCacheDir, "TestInvalidMetadataForgemodule")

	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		resolvePuppetfile(pfm)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() && strings.Contains(string(out), "WARNING: calculated sha256sum 59adaf8c4ab90ab629abcd8e965b6bdd28a022cf408e4e74b7294b47ce11644a for /tmp/forge_cachepuppetlabs-ntp-6.0.0.tar.gz does not match expected sha256sum a988a172a3edde6ac2a26d0e893faa88d37bc47465afc50d55225a036906c944") {
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
	got := readPuppetfile("tests/TestReadPuppetfile", "", "test", false, false)
	//fmt.Println(got.forgeModules["apt"].moduleDir)
	if "external_modules/" != got.forgeModules["apt"].moduleDir {
		t.Error("Expected 'external_modules/' for module dir, but got", got.forgeModules["apt"].moduleDir)
	}
	if "modules/" != got.gitModules["another_module"].moduleDir {
		t.Error("Expected 'modules/' for module dir, but got", got.gitModules["another_module"].moduleDir)
	}
	moduleDirParam = "foobar/"
	got = readPuppetfile("tests/TestReadPuppetfile", "", "test", false, false)
	if "foobar/" != got.forgeModules["apt"].moduleDir {
		t.Error("Expected '", moduleDirParam, "' for module dir, but got", got.forgeModules["apt"].moduleDir)
	}
	moduleDirParam = ""
}

func TestResolvConfigExitIfUnreachable(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigExitIfUnreachable.yaml")
	purgeDir(config.CacheDir, "TestResolvConfigExitIfUnreachable()")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		resolvePuppetEnvironment("single", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 1 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 1)
	}
	//fmt.Println(string(out))
	if !strings.Contains(string(out), "WARN: git repository git://github.com/xorpaul/g10k-environment-unavailable.git does not exist or is unreachable at this moment!\nWARNING: Could not resolve git repository in source 'example' (git://github.com/xorpaul/g10k-environment-unavailable.git)") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
}

func TestResolvConfigExitIfUnreachableFalse(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigExitIfUnreachableFalse.yaml")
	purgeDir(config.CacheDir, "TestResolvConfigExitIfUnreachableFalse()")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		resolvePuppetEnvironment("single", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	if !strings.Contains(string(out), "WARN: git repository git://github.com/xorpaul/g10k-environment-unavailable.git does not exist or is unreachable at this moment!\nWARNING: Could not resolve git repository in source 'example' (git://github.com/xorpaul/g10k-environment-unavailable.git)") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
}

func TestConfigUseCacheFallback(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/" + funcName + ".yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		resolvePuppetEnvironment("single_fail", false, "")
		return
	}

	// get the module to cache it
	doMirrorOrUpdate("https://github.com/puppetlabs/puppetlabs-firewall.git", "/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git/", "false", false, 0)

	// rename the cached module dir to match the otherwise failing single_fail env
	unresolvableGitDir := "/tmp/g10k/modules/https-__.com_puppetlabs_puppetlabs-firewall.git/"
	purgeDir(unresolvableGitDir, funcName)
	purgeDir("/tmp/example/single_fail", funcName)
	err := os.Rename("/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git/", unresolvableGitDir)
	if err != nil {
		t.Error(err)
	}

	// change the git remote url to something that does not resolv https://.com/...
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

	if 0 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	if !strings.Contains(string(out), "WARN: git repository https://.com/puppetlabs/puppetlabs-firewall.git does not exist or is unreachable at this moment!\nWARN: Trying to use cache for https://.com/puppetlabs/puppetlabs-firewall.git git repository") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
	if !fileExists("/tmp/example/single_fail/modules/firewall/metadata.json") {
		t.Errorf("terminated with the correct exit code and the correct output, but the resulting module was missing")
	}
}

func TestConfigUseCacheFallbackFalse(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/" + funcName + ".yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		resolvePuppetEnvironment("single_fail", false, "")
		return
	}

	// get the module to cache it
	doMirrorOrUpdate("https://github.com/puppetlabs/puppetlabs-firewall.git", "/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git/", "false", false, 0)

	// rename the cached module dir to match the otherwise failing single_fail env
	unresolvableGitDir := "/tmp/g10k/modules/https-__.com_puppetlabs_puppetlabs-firewall.git/"
	purgeDir(unresolvableGitDir, funcName)
	purgeDir("/tmp/example/single_fail", funcName)
	err := os.Rename("/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git/", unresolvableGitDir)
	if err != nil {
		t.Error(err)
	}

	// change the git remote url to something that does not resolv https://.com/...
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

	if 1 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 1)
	}
	//fmt.Println(string(out))
	if !strings.Contains(string(out), "executeCommand(): git command failed: git --git-dir /tmp/g10k/modules/https-__.com_puppetlabs_puppetlabs-firewall.git remote update --prune exit status 1\nOutput: Fetching origin\nfatal: unable to access 'https://.com/puppetlabs/puppetlabs-firewall.git/': Could") {
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
		resolvePuppetEnvironment("single_fail_forge", false, "")
		return
	}
	fm := ForgeModule{version: "1.9.0", author: "puppetlabs", name: "firewall"}
	config.Forge.Baseurl = "https://forgeapi.puppetlabs.com"
	downloadForgeModule("puppetlabs-firewall", "1.9.0", fm, 1)

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	//fmt.Println(string(out))
	if !strings.Contains(string(out), "Forge API error, trying to use cache for module puppetlabs/puppetlabs-firewall\nUsing cached version 1.9.0 for puppetlabs-firewall-latest") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}
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
		resolvePuppetEnvironment("single_fail_forge", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 1 != exitCode {
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
		resolvePuppetEnvironment("install_path", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
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
		resolvePuppetEnvironment("install_path", false, "")
		resolvePuppetEnvironment("install_path", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
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
		resolvePuppetEnvironment("single_module", false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	resolvePuppetEnvironment("single_module", false, "")
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

	if 0 != exitCode {
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
		resolvePuppetEnvironment("single_module", false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	resolvePuppetEnvironment("single_module", false, "")
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

	if 0 != exitCode {
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
		resolvePuppetEnvironment("fallback", false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	resolvePuppetEnvironment("fallback", false, "")
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

	if 0 != exitCode {
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
		resolvePuppetEnvironment("default_branch", false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	resolvePuppetEnvironment("default_branch", false, "")
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

	if 0 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	if !strings.Contains(string(out), "Trying to resolve /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apache.git with branch default_branch") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !strings.Contains(string(out), "Executing git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apache.git rev-parse --verify 'master^{object}' took") {
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
	testDir := "/tmp/example/foobar_control_branch_foobar/modules/g10k_testmodule/"
	initFile := testDir + "manifests/init.pp"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		resolvePuppetEnvironment("control_branch_foobar", false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	resolvePuppetEnvironment("control_branch_foobar", false, "")
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

	if 0 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	if !strings.Contains(string(out), "Trying to resolve /tmp/g10k/modules/https-__github.com_xorpaul_g10k_testmodule.git with branch control_branch_foobar") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !strings.Contains(string(out), "Executing git --git-dir /tmp/g10k/modules/https-__github.com_xorpaul_g10k_testmodule.git rev-parse --verify 'control_branch_foobar^{object}' took") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	branchFile := testDir + "MODULEBRANCHNAME_IS_control_branch_foobar"
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
	metadataFile := apacheDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		resolvePuppetEnvironment("control_branch_default", false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	resolvePuppetEnvironment("control_branch_default", false, "")
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

	if 0 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	if !strings.Contains(string(out), "Trying to resolve /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apache.git with branch control_branch_default") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s", string(out))
	}

	if !strings.Contains(string(out), "Executing git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apache.git rev-parse --verify 'master^{object}' took") {
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
	config = readConfigfile("tests/" + funcName + ".yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		resolvePuppetEnvironment("single_git", false, "")
		return
	}

	localGitRepoDir := "/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git/"
	purgeDir(localGitRepoDir, funcName)

	// get the module to cache it
	doMirrorOrUpdate("https://github.com/puppetlabs/puppetlabs-firewall.git", localGitRepoDir, "false", false, 0)

	// corrupt the local git module repository

	matches, _ := filepath.Glob(localGitRepoDir + "objects/pack/*.idx")
	for _, m := range matches {
		if err := os.RemoveAll(m); err != nil {
			t.Error("Error: deleting Git *.idx file to corrupt the local Git repository")
		}
		f, _ := os.Create(m)
		defer f.Close()
		f.WriteString("foobar")
		f.Sync()
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v", exitCode, 0)
	}
	//fmt.Println(string(out))
	if !strings.Contains(string(out), "WARN: git command failed: git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git remote update --prune deleting local cached repository and retrying...") {
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
		debug = true
		resolvePuppetEnvironment("local_modules", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}
	//fmt.Println(string(out))

	if !strings.Contains(string(out), "Need to sync /tmp/example/foobar_local_modules/modules/stdlib") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing 1. out: %s", string(out))
	}

	if !strings.Contains(string(out), "Not deleting /tmp/example/foobar_local_modules/modules/localstuff as it is declared as a local module") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing 2. out: %s", string(out))
	}

	if !strings.Contains(string(out), "Not deleting /tmp/example/foobar_local_modules/modules/localstuff2 as it is declared as a local module") {
		t.Errorf("terminated with the correct exit code, but the expected output was missing 3. out: %s", string(out))
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
		resolvePuppetEnvironment("invalid_git_object", false, "")
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
	if 1 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 1, string(out))
	}

	expectingString := "executeCommand(): git command failed: git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git rev-parse --verify '0000000000000000000000000000000000000000^{object}' exit status 128"
	if !strings.Contains(string(out), expectingString) {
		t.Errorf("terminated with the correct exit code, but the expected output was missing. out: %s\nExpecting string: %s", string(out), expectingString)
	}

	moduleParam = ""
	debug = false

}

func TestUnTarPreserveTimestamp(t *testing.T) {
	purgeDir("/tmp/example/", "TestUnTarPreserveTimestamp()")
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigPrefix.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		resolvePuppetEnvironment("master", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
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
		resolvePuppetEnvironment("fallback", false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	resolvePuppetEnvironment("fallback", false, "")
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

	if 0 != exitCode {
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
		resolvePuppetEnvironment("fallback", false, "")
		return
	}
	purgeDir("/tmp/example", funcName)
	resolvePuppetEnvironment("fallback", false, "")
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

	if 0 != exitCode {
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

func TestAutoCorrectEnvironmentNames(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/" + funcName + ".yaml")
	resolvePuppetEnvironment("single_autocorrect-%-fooo", false, "")

	firewallDir := "/tmp/example/single_autocorrect___fooo/modules/firewall"
	metadataFile := firewallDir + "/metadata.json"
	if !fileExists(metadataFile) {
		t.Errorf("expected module metadata.json is missing %s", metadataFile)
	}

	purgeDir("/tmp/example", funcName)
	moduleParam = ""
	debug = false

}

func TestAutoCorrectEnvironmentNamesDefault(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/" + funcName + ".yaml")
	resolvePuppetEnvironment("single_autocorrect-%-fooo", false, "")

	firewallDir := "/tmp/example/single_autocorrect-%-fooo/modules/firewall"
	metadataFile := firewallDir + "/metadata.json"
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
	config = readConfigfile("tests/" + funcName + ".yaml")
	firewallDir := "/tmp/example/single_autocorrect___fooo/modules/firewall"
	metadataFile := firewallDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		resolvePuppetEnvironment("single_autocorrect-%-fooo", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
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
	config = readConfigfile("tests/" + funcName + ".yaml")
	firewallDir := "/tmp/example/single_autocorrect-%-fooo/modules/firewall"
	metadataFile := firewallDir + "/metadata.json"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		resolvePuppetEnvironment("single_autocorrect-%-fooo", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
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
		resolvePuppetEnvironment("single_cache", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
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

	resolvePuppetEnvironment("single_cache", false, "")
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
		resolvePuppetEnvironment("single", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
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
		resolvePuppetEnvironment("", false, "")
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

	if 0 != exitCode {
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
		resolvePuppetEnvironment("", false, "")
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

	if 0 != exitCode {
		t.Errorf("terminated with %v, but we expected exit status %v Output: %s", exitCode, 0, string(out))
	}

	if !fileExists(postrunLogfile) {
		t.Errorf("postrun logfile file missing: %s", postrunLogfile)
	}

	content, _ := ioutil.ReadFile(postrunLogfile)

	expectedLines := []string{
		"postrun command wrapper script received argument: /tmp/example/example_master/",
		"postrun command wrapper script received argument: /tmp/example/example_foobar/",
		"postrun command wrapper script received argument: /tmp/example/example_foobar/modules/systemd/",
		"postrun command wrapper script received argument: /tmp/example/example_master/modules/systemd/",
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
		resolvePuppetEnvironment("multiple_moduledir", false, "")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		exitCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if 0 != exitCode {
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

	resolvePuppetEnvironment("multiple_moduledir", false, "")

	if isDir(unmanagedModule1) {
		t.Errorf("Unmanaged Module directory 1 is still there and should not be: %s", unmanagedModule1)
	}

	if isDir(unmanagedModule2) {
		t.Errorf("Unmanaged Module directory 2 is still there and should not be: %s", unmanagedModule2)
	}

	purgeDir("/tmp/example", funcName)
	purgeDir("/tmp/g10k", funcName)
	moduleParam = ""
	debug = false
}

func TestFailedGit(t *testing.T) {
	quiet = true
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	config = readConfigfile("tests/TestConfigRetryGitCommands.yaml")
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		resolvePuppetEnvironment("single_fail", false, "")
		return
	}

	// get the module to cache it
	gitDir := "/tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-firewall.git/"
	gitUrl := "https://github.com/puppetlabs/puppetlabs-firewall.git"
	purgeDir(gitDir, funcName)
	doMirrorOrUpdate(gitUrl, gitDir, "false", false, 0)

	// change the git remote url to something that does not resolv https://.com/...
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

	if 1 != exitCode {
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
		resolvePuppetEnvironment("single", false, "")
		return
	} else {
		purgeDir(cacheDir, funcName)
		// create cacheDir and make sure the cachedir does not have write permissions
		if err := os.MkdirAll(cacheDir, 0444); err != nil {
			Fatalf("checkDirAndCreate(): Error: failed to create directory: " + cacheDir + " Error: " + err.Error())
		}
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
		resolvePuppetEnvironment("single", false, "")
		return
	} else {
		createOrPurgeDir("/tmp/example/single/stale_directory_that_should_be_purged", funcName)
		createOrPurgeDir("/tmp/example/single/.resource_types", funcName)
		f, _ := os.Create("/tmp/example/single/.latest_revision")
		defer f.Close()
		f.WriteString("foobar")
		f.Sync()
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
		"DEBUG checkForStaleContent(): additional purge whitelist items: .latest_revision .resource_types",
		"Removing unmanaged path /tmp/example/single/stale_directory_that_should_be_purged",
		"DEBUG purgeDir(): Trying to remove: /tmp/example/single/stale_directory_that_should_be_purged called from checkForStaleContent()",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	if !fileExists("/tmp/example/single/.resource_types") ||
		!fileExists("/tmp/example/single/.latest_revision") {
		t.Errorf("purge whitelist item was purged!")
	}

	if !fileExists("/tmp/example/single/external_modules/inifile/README.md") {
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
		resolvePuppetEnvironment("single", false, "")
		return
	} else {
		createOrPurgeDir("/tmp/example/single/stale_directory_that_should_be_purged", funcName)
		createOrPurgeDir("/tmp/example/single/stale_directory_that_should_be_purged2", funcName)
		f, _ := os.Create("/tmp/example/single/stale_directory_that_should_be_purged/stale_file")
		defer f.Close()
		f.WriteString("foobar")
		f.Sync()
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
		resolvePuppetEnvironment("", false, "")
		return
	} else {
		createOrPurgeDir("/tmp/full/full_stale/stale_directory_that_should_be_purged", funcName)
		createOrPurgeDir("/tmp/full/full_stale/stale_dir", funcName)
		f, _ := os.Create("/tmp/full/full_stale/stale_dir/stale_file")
		defer f.Close()
		f.WriteString("foobar")
		f.Sync()
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
		resolvePuppetEnvironment("", false, "")
		return
	} else {
		createOrPurgeDir("/tmp/full/full_master/modules/stale_module_directory_that_should_not_be_purged", funcName)
		createOrPurgeDir("/tmp/full/full_master/stale_directory_that_should_not_be_purged", funcName)
		createOrPurgeDir("/tmp/full/full_stale/stale_directory_that_should_be_purged", funcName)
		createOrPurgeDir("/tmp/full/full_stale/stale_dir", funcName)
		f, _ := os.Create("/tmp/full/full_stale/stale_dir/stale_file")
		defer f.Close()
		f.WriteString("foobar")
		f.Sync()
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
		resolvePuppetEnvironment("", false, "")
		return
	} else {
		createOrPurgeDir("/tmp/full/full_master/modules/stale_module_directory_that_should_be_purged", funcName)
		createOrPurgeDir("/tmp/full/full_master/stale_directory_that_should_not_be_purged", funcName)
		createOrPurgeDir("/tmp/full/full_stale/stale_directory_that_should_not_be_purged", funcName)
		createOrPurgeDir("/tmp/full/full_stale/stale_dir", funcName)
		f, _ := os.Create("/tmp/full/full_stale/stale_dir/stale_file")
		defer f.Close()
		f.WriteString("foobar")
		f.Sync()
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
		resolvePuppetEnvironment("", false, "")
		return
	} else {
		createOrPurgeDir("/tmp/full/full_master/modules/stale_module_directory_that_should_not_be_purged", funcName)
		createOrPurgeDir("/tmp/full/full_master/stale_directory_that_should_not_be_purged", funcName)
		createOrPurgeDir("/tmp/full/full_stale/stale_directory_that_should_be_purged", funcName)
		createOrPurgeDir("/tmp/full/full_stale/stale_dir", funcName)
		f, _ := os.Create("/tmp/full/full_stale/stale_dir/stale_file")
		defer f.Close()
		f.WriteString("foobar")
		f.Sync()
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
		resolvePuppetEnvironment("", false, "")
		return
	} else {
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

func TestRespectPrefixInBranchMode(t *testing.T) {
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	cacheDir := "/tmp/g10k"
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		debug = true
		config = readConfigfile("tests/TestConfigFullworkingAndExample.yaml")
		resolvePuppetEnvironment("full_master", false, "")
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
		"DEBUG resolvePuppetfile(): Resolving full_master",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(string(out), expectedLine) {
			t.Errorf("Could not find expected line '" + expectedLine + "' in debug output")
		}
	}

	if fileExists("/tmp/out/example_master/") {
		t.Errorf("Puppet environment example_master should not have been deployed, with branch parameter set to full_master")
	}

	expectedFiles := []string{
		"/tmp/out/full_master/modules/stdlib/metadata.json",
	}

	for _, expectedFile := range expectedFiles {
		if !fileExists(expectedFile) {
			t.Errorf("Puppet environment full_master seems not to have been populated " + expectedFile)
		}
	}

	purgeDir(cacheDir, funcName)
	purgeDir("/tmp/out", funcName)
}
