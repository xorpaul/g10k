package main

import (
		"regexp"
		"testing"
)


func TestPreparePuppetfile(t *testing.T) {
	expected := regexp.MustCompile("(moduledir 'external_modules'\nmod 'puppetlabs/ntp')")
	got := preparePuppetfile("tests/TestPreparePuppetfile")

	if ! expected.MatchString(got) {
		t.Error("Expected", expected, "got", got)
	}
}
