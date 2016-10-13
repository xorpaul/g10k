package main

import (
	"regexp"
	"testing"
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
