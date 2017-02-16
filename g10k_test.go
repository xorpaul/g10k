package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func TestForgeChecksum(t *testing.T) {
	expectedFmm := ForgeModule{md5sum: "8a8c741978e578921e489774f05e9a65", fileSize: 57358}
	fmm := getMetadataForgeModule(ForgeModule{version: "2.2.0", name: "apt",
		author: "puppetlabs", baseUrl: "https://forgeapi.puppetlabs.com"})

	if fmm.md5sum != expectedFmm.md5sum {
		t.Error("Expected md5sum", expectedFmm.md5sum, "got", fmm.md5sum)
	}

	if fmm.fileSize != expectedFmm.fileSize {
		t.Error("Expected fileSize", expectedFmm.fileSize, "got", fmm.fileSize)
	}
}

func TestConfigPrefix(t *testing.T) {
	got := readConfigfile("tests/TestConfigPrefix.yaml")

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-environment.git",
		Basedir: "/tmp/example/", Prefix: "foobar", PrivateKey: ""}

	expected := ConfigSettings{
		CacheDir: "/tmp/g10k/", ForgeCacheDir: "/tmp/g10k/forge/",
		ModulesCacheDir: "/tmp/g10k/modules/", EnvCacheDir: "/tmp/g10k/environments/",
		Git:     Git{privateKey: "", username: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppetlabs.com"},
		Sources: s, Timeout: 5}

	if !reflect.DeepEqual(got, expected) {
		t.Error("Expected ConfigSettings:", expected, ", but got ConfigSettings:", got)
	}
}

func TestConfigForceForgeVersions(t *testing.T) {
	got := readConfigfile("tests/TestConfigForceForgeVersions.yaml")

	s := make(map[string]Source)
	s["example"] = Source{Remote: "https://github.com/xorpaul/g10k-environment.git",
		Basedir: "/tmp/example/", Prefix: "foobar", PrivateKey: "", ForceForgeVersions: true}

	expected := ConfigSettings{
		CacheDir: "/tmp/g10k/", ForgeCacheDir: "/tmp/g10k/forge/",
		ModulesCacheDir: "/tmp/g10k/modules/", EnvCacheDir: "/tmp/g10k/environments/",
		Git:     Git{privateKey: "", username: ""},
		Forge:   Forge{Baseurl: "https://forgeapi.puppetlabs.com"},
		Sources: s, Timeout: 5}

	if !reflect.DeepEqual(got, expected) {
		t.Error("Expected ConfigSettings:", expected, ", but got ConfigSettings:", got)
	}
}

func TestInvalidFilesizeForgemodule(t *testing.T) {
	ts := spinUpFakeForge(t, "tests/fake-forge/invalid-filesize-puppetlabs-ntp-metadata.json")
	defer ts.Close()

	f := ForgeModule{version: "6.0.0", name: "ntp", author: "puppetlabs",
		baseUrl: ts.URL, sha256sum: "59adaf8c4ab90ab629abcd8e965b6bdd28a022cf408e4e74b7294b47ce11644a"}
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
		forgeBaseURL: f.baseUrl, workDir: "/tmp/test_test/"}
	pfm := make(map[string]Puppetfile)
	pfm["test"] = pf

	config = ConfigSettings{ForgeCacheDir: "/tmp/forge_cache"}
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
		t.Errorf("resolvePuppetfile() terminated with the correct exit code, but the expected output was missing")
	}

}

func TestInvalidMd5sumForgemodule(t *testing.T) {
	ts := spinUpFakeForge(t, "tests/fake-forge/invalid-md5sum-puppetlabs-ntp-metadata.json")
	defer ts.Close()
	f := ForgeModule{version: "6.0.0", name: "ntp", author: "puppetlabs",
		baseUrl: ts.URL, sha256sum: "a988a172a3edde6ac2a26d0e893faa88d37bc47465afc50d55225a036906c944"}
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
		forgeBaseURL: f.baseUrl, workDir: "/tmp/test_test/"}
	pfm := make(map[string]Puppetfile)
	pfm["test"] = pf

	config = ConfigSettings{ForgeCacheDir: "/tmp/forge_cache"}
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
		baseUrl: ts.URL, sha256sum: "a988a172a3edde6ac2a26d0e893faa88d37bc47465afc50d55225a036906c944"}
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
		forgeBaseURL: f.baseUrl, workDir: "/tmp/test_test/"}
	pfm := make(map[string]Puppetfile)
	pfm["test"] = pf

	config = ConfigSettings{ForgeCacheDir: "/tmp/forge_cache"}
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
	got := readPuppetfile("tests/TestReadPuppetfile", "", "test", false)
	if "external_modules" != got.moduleDir {
		t.Error("Expected 'external_modules' for module dir, but got", got.moduleDir)
	}
	moduleDirParam = "foobar"
	got = readPuppetfile("tests/TestReadPuppetfile", "", "test", false)
	if moduleDirParam != got.moduleDir {
		t.Error("Expected '", moduleDirParam, "' for module dir, but got", got.moduleDir)
	}
}
