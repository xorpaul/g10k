package main

import (
	"reflect"
	"regexp"
	"testing"
	"time"
)

func comparePuppetfile(a, b GitModule) bool {
	if &a == &b {
		return true
	}
	if a.git != b.git || a.link != b.link ||
		a.privateKey != b.privateKey ||
		a.branch != b.branch ||
		a.tag != b.tag ||
		a.commit != b.commit ||
		a.ref != b.ref ||
		a.link != b.link ||
		a.ignoreUnreachable != b.ignoreUnreachable {
		return false
	}
	if len(a.fallback) != len(b.fallback) {
		return false
	}
	for i, v := range a.fallback {
		if b.fallback[i] != v {
			return false
		}
	}
	return true
}

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

func TestFallbackPuppetfile(t *testing.T) {
	fallbackMapExample := make([]string, 1)
	fallbackMapExample[0] = "master"

	fallbackMapAnother := make([]string, 4)
	fallbackMapAnother[0] = "dev"
	fallbackMapAnother[1] = "qa"
	fallbackMapAnother[2] = "prelive"
	fallbackMapAnother[3] = "live"

	gm := make(map[string]GitModule)
	gm["example_module"] = GitModule{git: "git@somehost.com/foo/example-module.git",
		link: true, ignoreUnreachable: false, fallback: fallbackMapExample}
	gm["another_module"] = GitModule{git: "git@somehost.com/foo/another-module.git",
		link: true, ignoreUnreachable: false, fallback: fallbackMapAnother}

	expected := Puppetfile{moduleDir: "modules", gitModules: gm, source: "test"}
	got := readPuppetfile("tests/TestFallbackPuppetfile", "", "test")

	if !comparePuppetfile(got.gitModules["example_module"], expected.gitModules["example_module"]) {
		t.Error("Expected gitModules:", expected.gitModules["example_module"], ", but got gitModules:", got.gitModules["example_module"])
	}

	if !comparePuppetfile(got.gitModules["another_module"], expected.gitModules["another_module"]) {
		t.Error("Expected gitModules:", expected.gitModules["another_module"], ", but got gitModules:", got.gitModules["another_module"])
	}

}
