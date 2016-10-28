package main

import (
	"reflect"
	"regexp"
	"testing"
	"time"
)

func TestPreparePuppetfile(t *testing.T) {
	expected := regexp.MustCompile("(moduledir 'external_modules'\nmod 'puppetlabs/ntp')")
	got := preparePuppetfile("tests/TestPreparePuppetfile")

	if !expected.MatchString(got) {
		t.Error("Expected", expected, "got", got)
	}
}

func TestCommentPuppetfile(t *testing.T) {
	expected := regexp.MustCompile("mod 'sensu',\\s+:git => 'https://github.com/sensu/sensu-puppet.git',\\s+:commit => '8f4fc5780071c4895dec559eafc6030511b0caaa'")
	got := preparePuppetfile("tests/TestCommentPuppetfile")

	if !expected.MatchString(got) {
		t.Error("Expected", expected, "got", got)
	}
}

func TestForgeChecksum(t *testing.T) {
	expectedFmm := ForgeModule{hashSum: "8a8c741978e578921e489774f05e9a65", fileSize: 57358}
	fmm := getMetadataForgeModule(ForgeModule{version: "2.2.0", name: "apt", author: "puppetlabs", baseUrl: "https://forgeapi.puppetlabs.com"})

	if fmm.hashSum != expectedFmm.hashSum {
		t.Error("Expected hashSum", expectedFmm.hashSum, "got", fmm.hashSum)
	}

	if fmm.fileSize != expectedFmm.fileSize {
		t.Error("Expected fileSize", expectedFmm.fileSize, "got", fmm.fileSize)
	}
}

func TestForgeCacheTtlPuppetfile(t *testing.T) {
	expected := regexp.MustCompile("(moduledir 'external_modules'\nforge.cacheTtl 50m\n)")
	got := preparePuppetfile("tests/TestForgeCacheTtlPuppetfile")

	if !expected.MatchString(got) {
		t.Error("Expected", expected, "got", got)
	}

	expectedPuppetfile := Puppetfile{moduleDir: "external_modules", forgeCacheTtl: 50 * time.Minute}
	gotPuppetfile := readPuppetfile("tests/TestForgeCacheTtlPuppetfile", "", "test")

	if gotPuppetfile.forgeCacheTtl != expectedPuppetfile.forgeCacheTtl {
		t.Error("Expected for forgeCacheTtl", expectedPuppetfile.forgeCacheTtl, "got", gotPuppetfile.forgeCacheTtl)
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
