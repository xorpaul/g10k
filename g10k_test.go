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
	expected := regexp.MustCompile("mod 'sensu',\\s+:git => 'github.com/sensu/sensu-puppet.git',\\s+:commit => '8f4fc5780071c4895dec559eafc6030511b0caaa'")
	got := preparePuppetfile("tests/TestCommentPuppetfile")

	if !expected.MatchString(got) {
		t.Error("Expected", expected, "got", got)
	}
}
