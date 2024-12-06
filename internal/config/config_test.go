package config

import (
	"os"
	"strings"
	"testing"
)

func TestStripRubySymbols(t *testing.T) {
	data := []byte(`:cachedir: '/tmp/g10k'\nforge_cache_ttl: 24h`)
	want := []byte(`cachedir: '/tmp/g10k'\nforge_cache_ttl: 24h`)
	got := stripRubySymbols(data)
	if !strings.Contains(string(got), string(want)) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadConfigFileWithDeploy(t *testing.T) {
	// Create a temporary config file
	tmpfile, err := os.CreateTemp("", "config_test.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write sample config data to the temporary file
	configData := `
cachedir: '/tmp/g10k'
forge_cache_ttl: 24h
timeout: 300
ignore_unreachable_modules: true
maxworker: 50
maxextractworker: 20
use_cache_fallback: true
retry_git_commands: true
git_object_syntax_not_supported: true
postrun:
  - 'echo "Post run command"'
deploy:
  purge_levels:
  - deployment
  - puppetfile
  purge_allowlist:
  - /etc/puppetlabs/code/environments
  deployment_purge_allowlist:
  - /etc/puppetlabs/code/environments
  write_lock: "lock"
  generate_types: true
  puppet_path: "/opt/puppetlabs/bin/puppet"
  purge_skiplist:
  - /etc/puppetlabs/code/environments
`
	if _, err := tmpfile.Write([]byte(configData)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Call ReadConfigFile with the path to the temporary file
	config := ReadConfigFile(tmpfile.Name())

	// Verify the returned ConfigSettings struct
	if config.CacheDir != "/tmp/g10k" {
		t.Errorf("CacheDir = %q, want %q", config.CacheDir, "/tmp/g10k")
	}
	if config.ForgeCacheTTLString != "24h" {
		t.Errorf("ForgeCacheTTLString = %q, want %q", config.ForgeCacheTTLString, "24h")
	}
	if config.Timeout != 300 {
		t.Errorf("Timeout = %d, want %d", config.Timeout, 300)
	}
	if !config.IgnoreUnreachableModules {
		t.Errorf("IgnoreUnreachableModules = %v, want %v", config.IgnoreUnreachableModules, true)
	}
	if config.Maxworker != 50 {
		t.Errorf("Maxworker = %d, want %d", config.Maxworker, 50)
	}
	if config.MaxExtractworker != 20 {
		t.Errorf("MaxExtractworker = %d, want %d", config.MaxExtractworker, 20)
	}
	if !config.UseCacheFallback {
		t.Errorf("UseCacheFallback = %v, want %v", config.UseCacheFallback, true)
	}
	if !config.RetryGitCommands {
		t.Errorf("RetryGitCommands = %v, want %v", config.RetryGitCommands, true)
	}
	if !config.GitObjectSyntaxNotSupported {
		t.Errorf("GitObjectSyntaxNotSupported = %v, want %v", config.GitObjectSyntaxNotSupported, true)
	}
	if len(config.PostRunCommand) != 1 || config.PostRunCommand[0] != "echo \"Post run command\"" {
		t.Errorf("PostRunCommand = %v, want %v", config.PostRunCommand, []string{"echo \"Post run command\""})
	}
	if config.PurgeLevels[0] != "deployment" || config.PurgeLevels[1] != "puppetfile" {
		t.Errorf("PurgeLevels = %v, want %v", config.PurgeLevels, []string{"deployment", "puppetfile"})
	}
	if config.PurgeAllowList[0] != "/etc/puppetlabs/code/environments" {
		t.Errorf("PurgeAllowList = %v, want %v", config.PurgeAllowList, []string{"/etc/puppetlabs/code/environments"})
	}
	if config.DeploymentPurgeAllowList[0] != "/etc/puppetlabs/code/environments" {
		t.Errorf("DeploymentPurgeAllowList = %v, want %v", config.DeploymentPurgeAllowList, []string{"/etc/puppetlabs/code/environments"})
	}
	if config.WriteLock != "lock" {
		t.Errorf("WriteLock = %q, want %q", config.WriteLock, "lock")
	}
	if !config.GenerateTypes {
		t.Errorf("GenerateTypes = %v, want %v", config.GenerateTypes, true)
	}
	if config.PuppetPath != "/opt/puppetlabs/bin/puppet" {
		t.Errorf("PuppetPath = %q, want %q", config.PuppetPath, "/opt/puppetlabs/bin/puppet")
	}
	if config.PurgeSkiplist[0] != "/etc/puppetlabs/code/environments" {
		t.Errorf("PurgeSkiplist = %v, want %v", config.Deploy.PurgeSkiplist, []string{"/etc/puppetlabs/code/environments"})
	}
}

func TestReadConfigFileWithoutDeploy(t *testing.T) {
	// Create a temporary config file
	tmpfile, err := os.CreateTemp("", "config_test.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write sample config data to the temporary file
	configData := `
cachedir: '/tmp/g10k'
forge_cache_ttl: 24h
timeout: 300
ignore_unreachable_modules: true
maxworker: 50
maxextractworker: 20
use_cache_fallback: true
retry_git_commands: true
git_object_syntax_not_supported: true
postrun:
  - 'echo "Post run command"'
purge_levels:
- deployment
- puppetfile
purge_allowlist:
- /etc/puppetlabs/code/environments
deployment_purge_allowlist:
- /etc/puppetlabs/code/environments
write_lock: "lock"
generate_types: true
puppet_path: "/opt/puppetlabs/bin/puppet"
purge_skiplist:
- /etc/puppetlabs/code/environments
`
	if _, err := tmpfile.Write([]byte(configData)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Call ReadConfigFile with the path to the temporary file
	config := ReadConfigFile(tmpfile.Name())

	// Verify the returned ConfigSettings struct
	if config.CacheDir != "/tmp/g10k" {
		t.Errorf("CacheDir = %q, want %q", config.CacheDir, "/tmp/g10k")
	}
	if config.ForgeCacheTTLString != "24h" {
		t.Errorf("ForgeCacheTTLString = %q, want %q", config.ForgeCacheTTLString, "24h")
	}
	if config.Timeout != 300 {
		t.Errorf("Timeout = %d, want %d", config.Timeout, 300)
	}
	if !config.IgnoreUnreachableModules {
		t.Errorf("IgnoreUnreachableModules = %v, want %v", config.IgnoreUnreachableModules, true)
	}
	if config.Maxworker != 50 {
		t.Errorf("Maxworker = %d, want %d", config.Maxworker, 50)
	}
	if config.MaxExtractworker != 20 {
		t.Errorf("MaxExtractworker = %d, want %d", config.MaxExtractworker, 20)
	}
	if !config.UseCacheFallback {
		t.Errorf("UseCacheFallback = %v, want %v", config.UseCacheFallback, true)
	}
	if !config.RetryGitCommands {
		t.Errorf("RetryGitCommands = %v, want %v", config.RetryGitCommands, true)
	}
	if !config.GitObjectSyntaxNotSupported {
		t.Errorf("GitObjectSyntaxNotSupported = %v, want %v", config.GitObjectSyntaxNotSupported, true)
	}
	if len(config.PostRunCommand) != 1 || config.PostRunCommand[0] != "echo \"Post run command\"" {
		t.Errorf("PostRunCommand = %v, want %v", config.PostRunCommand, []string{"echo \"Post run command\""})
	}
	if config.PurgeLevels[0] != "deployment" || config.PurgeLevels[1] != "puppetfile" {
		t.Errorf("PurgeLevels = %v, want %v", config.PurgeLevels, []string{"deployment", "puppetfile"})
	}
	if config.PurgeAllowList[0] != "/etc/puppetlabs/code/environments" {
		t.Errorf("PurgeAllowList = %v, want %v", config.PurgeAllowList, []string{"/etc/puppetlabs/code/environments"})
	}
	if config.DeploymentPurgeAllowList[0] != "/etc/puppetlabs/code/environments" {
		t.Errorf("DeploymentPurgeAllowList = %v, want %v", config.DeploymentPurgeAllowList, []string{"/etc/puppetlabs/code/environments"})
	}
	if config.WriteLock != "lock" {
		t.Errorf("WriteLock = %q, want %q", config.WriteLock, "lock")
	}
	if !config.GenerateTypes {
		t.Errorf("GenerateTypes = %v, want %v", config.GenerateTypes, true)
	}
	if config.PuppetPath != "/opt/puppetlabs/bin/puppet" {
		t.Errorf("PuppetPath = %q, want %q", config.PuppetPath, "/opt/puppetlabs/bin/puppet")
	}
	if config.PurgeSkiplist[0] != "/etc/puppetlabs/code/environments" {
		t.Errorf("PurgeSkiplist = %v, want %v", config.Deploy.PurgeSkiplist, []string{"/etc/puppetlabs/code/environments"})
	}
}
