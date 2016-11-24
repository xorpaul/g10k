package main

import (
	"reflect"
	"testing"
)

func TestForgeChecksum(t *testing.T) {
	t.Parallel()
	expectedFmm := ForgeModule{md5sum: "8a8c741978e578921e489774f05e9a65", fileSize: 57358}
	fmm := getMetadataForgeModule(ForgeModule{version: "2.2.0", name: "apt", author: "puppetlabs", baseUrl: "https://forgeapi.puppetlabs.com"})

	if fmm.md5sum != expectedFmm.md5sum {
		t.Error("Expected md5sum", expectedFmm.md5sum, "got", fmm.md5sum)
	}

	if fmm.fileSize != expectedFmm.fileSize {
		t.Error("Expected fileSize", expectedFmm.fileSize, "got", fmm.fileSize)
	}
}

func TestConfigPrefix(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
