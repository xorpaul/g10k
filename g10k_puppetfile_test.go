package main

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

func equalPuppetfile(a, b Puppetfile) bool {
	if &a == &b {
		return true
	}
	if a.moduleDir != b.moduleDir || a.forgeBaseURL != b.forgeBaseURL ||
		a.forgeCacheTtl != b.forgeCacheTtl ||
		a.privateKey != b.privateKey ||
		a.source != b.source {
		return false
	}

	if len(a.gitModules) != len(b.gitModules) ||
		len(a.forgeModules) != len(b.forgeModules) {
		return false
	}

	for gitModuleName, gm := range a.gitModules {
		if _, ok := b.gitModules[gitModuleName]; !ok {
			return false
		}
		if !equalGitModule(gm, b.gitModules[gitModuleName]) {
			return false
		}
	}

	for forgeModuleName, fm := range a.forgeModules {
		if _, ok := b.forgeModules[forgeModuleName]; !ok {
			return false
		}
		//fmt.Println("checking Forge module: ", forgeModuleName, fm)
		if !equalForgeModule(fm, b.forgeModules[forgeModuleName]) {
			return false
		}
	}

	return true
}

func equalForgeModule(a, b ForgeModule) bool {
	if &a == &b {
		return true
	}
	if a.author != b.author || a.name != b.name ||
		a.version != b.version ||
		a.hashSum != b.hashSum ||
		a.fileSize != b.fileSize ||
		a.baseUrl != b.baseUrl ||
		a.cacheTtl != b.cacheTtl {
		return false
	}
	return true
}

func equalGitModule(a, b GitModule) bool {
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
	t.Parallel()
	expected := regexp.MustCompile("(moduledir 'external_modules'\nmod 'puppetlabs/ntp')")
	got := preparePuppetfile("tests/TestPreparePuppetfile")

	if !expected.MatchString(got) {
		t.Error("Expected", expected, "got", got)
	}
}

func TestCommentPuppetfile(t *testing.T) {
	t.Parallel()
	expected := regexp.MustCompile("mod 'sensu',\\s*:git => 'https://github.com/sensu/sensu-puppet.git',\\s*:commit => '8f4fc5780071c4895dec559eafc6030511b0caaa'")
	got := preparePuppetfile("tests/TestCommentPuppetfile")

	if !expected.MatchString(got) {
		t.Error("Expected", expected, "got", got)
	}
}

func TestReadPuppetfile(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	got := readPuppetfile("tests/"+funcName, "", "test", false)

	fallbackMapExample := make([]string, 1)
	fallbackMapExample[0] = "master"

	fallbackMapExampleFull := make([]string, 3)
	fallbackMapExampleFull[0] = "b"
	fallbackMapExampleFull[1] = "a"
	fallbackMapExampleFull[2] = "r"

	fallbackMapAnother := make([]string, 4)
	fallbackMapAnother[0] = "dev"
	fallbackMapAnother[1] = "qa"
	fallbackMapAnother[2] = "prelive"
	fallbackMapAnother[3] = "live"

	gm := make(map[string]GitModule)
	gm["sensu"] = GitModule{git: "https://github.com/sensu/sensu-puppet.git",
		commit: "8f4fc5780071c4895dec559eafc6030511b0caaa", ignoreUnreachable: false}
	gm["example_module"] = GitModule{git: "git@somehost.com/foo/example-module.git",
		link: true, ignoreUnreachable: false, fallback: fallbackMapExample}
	gm["another_module"] = GitModule{git: "git@somehost.com/foo/another-module.git",
		link: true, ignoreUnreachable: false, fallback: fallbackMapAnother}
	gm["example_module_full"] = GitModule{git: "git@somehost.com/foo/example-module.git",
		branch: "foo", ignoreUnreachable: true, fallback: fallbackMapExampleFull}

	fm := make(map[string]ForgeModule)
	fm["puppetlabs/apt"] = ForgeModule{version: "2.3.0", author: "puppetlabs", name: "apt"}
	fm["puppetlabs/ntp"] = ForgeModule{version: "present", author: "puppetlabs", name: "ntp"}
	fm["puppetlabs/stdlib"] = ForgeModule{version: "latest", author: "puppetlabs", name: "stdlib"}

	expected := Puppetfile{moduleDir: "external_modules", gitModules: gm, forgeModules: fm, source: "test", forgeCacheTtl: time.Duration(50 * time.Minute), forgeBaseURL: "foobar"}

	if !equalPuppetfile(got, expected) {
		t.Error("Expected Puppetfile:", expected, ", but got Puppetfile:", got)
	}
}

func TestFallbackPuppetfile(t *testing.T) {
	t.Parallel()
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
		branch: "master", ignoreUnreachable: false, fallback: fallbackMapAnother}

	expected := Puppetfile{moduleDir: "modules", gitModules: gm, source: "test"}
	got := readPuppetfile("tests/TestFallbackPuppetfile", "", "test", false)

	if !equalGitModule(got.gitModules["example_module"], expected.gitModules["example_module"]) {
		t.Error("Expected gitModules:", expected.gitModules["example_module"], ", but got gitModules:", got.gitModules["example_module"])
	}

	if !equalGitModule(got.gitModules["another_module"], expected.gitModules["another_module"]) {
		t.Error("Expected gitModules:", expected.gitModules["another_module"], ", but got gitModules:", got.gitModules["another_module"])
	}
}

func TestForgeCacheTtlPuppetfile(t *testing.T) {
	t.Parallel()
	expected := regexp.MustCompile("(moduledir 'external_modules'\nforge.cacheTtl 50m\n)")
	got := preparePuppetfile("tests/TestForgeCacheTtlPuppetfile")

	if !expected.MatchString(got) {
		t.Error("Expected", expected, "got", got)
	}

	expectedPuppetfile := Puppetfile{moduleDir: "external_modules", forgeCacheTtl: 50 * time.Minute}
	gotPuppetfile := readPuppetfile("tests/TestForgeCacheTtlPuppetfile", "", "test", false)

	if gotPuppetfile.forgeCacheTtl != expectedPuppetfile.forgeCacheTtl {
		t.Error("Expected for forgeCacheTtl", expectedPuppetfile.forgeCacheTtl, "got", gotPuppetfile.forgeCacheTtl)
	}

}

func TestForceForgeVersionsPuppetfile(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", true)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)
}

func TestForceForgeVersionsPuppetfileCorrect(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", true)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 0", err)
	}
	return
}

func TestReadPuppetfileDuplicateGitAttribute(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)
}

func TestReadPuppetfileTrailingComma(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}

func TestReadPuppetfileInvalidForgeModuleName(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}

func TestReadPuppetfileDuplicateForgeModule(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}

func TestReadPuppetfileMissingGitAttribute(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}

func TestReadPuppetfileTooManyGitAttributes(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}

func TestReadPuppetfileConflictingGitAttributesTag(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}

func TestReadPuppetfileConflictingGitAttributesBranch(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}

func TestReadPuppetfileConflictingGitAttributesCommit(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}

func TestReadPuppetfileConflictingGitAttributesRef(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}

func TestReadPuppetfileIgnoreUnreachable(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}

func TestReadPuppetfileForgeCacheTtl(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}

func TestReadPuppetfileLink(t *testing.T) {
	t.Parallel()
	funcName := strings.Split(funcName(), ".")[len(strings.Split(funcName(), "."))-1]
	if os.Getenv("TEST_FOR_CRASH_"+funcName) == "1" {
		readPuppetfile("tests/"+funcName, "", "test", false)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run="+funcName+"$")
	cmd.Env = append(os.Environ(), "TEST_FOR_CRASH_"+funcName+"=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Errorf("readPuppetfile() terminated with %v, but we expected exit status 1", err)

}
