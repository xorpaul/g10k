[![Build Status](https://travis-ci.org/xorpaul/g10k.svg?branch=master)](https://travis-ci.org/xorpaul/g10k) [![Go Report Card](https://goreportcard.com/badge/github.com/xorpaul/g10k)](https://goreportcard.com/report/github.com/xorpaul/g10k)
# g10k
My r10k fork written in Go, designed to work as a drop-in replacement* in place of [puppetlabs/r10k](https://github.com/puppetlabs/r10k).

### Why fork?
  - Lack of caching/version-pre-checking in current r10k implementation hurt perfomance beyond a certain # of modules per Puppetfile
  - We need distinct SSHKeys for each source in the r10k.yaml and 'rugged' never really wanted to play nice (fixed in r10k [2.2.0](https://github.com/puppetlabs/r10k/blob/master/CHANGELOG.mkd#220 ))
  - Good excuse to try Go ;)

### Changes breaking 'true' drop-in replacement capability
  - No SVN support
  - No 'local'-Modules support

### Non-breaking changes to r10k
  - Download/Cache each git Puppet Module repository and each Puppetlabs Forge Puppet Module for each respective version only once
  - Most things (git, forge, and copy operations) done in parallel over each branch
  - Optional support for different ssh keys for each source inside the r10k.yaml

### Pseudo "benchmark"


Using Puppetfile with 4 git repositories and 25 Forge modules
https://github.com/xorpaul/g10k-environment/blob/benchmark/Puppetfile

| 2016-10-14 | w/o cache | w/ cache |
| ------------ | ------------ | ------------- |
| r10k | 1m14s,1m18s,1m12s | 18s,17s,17s |
| g10k | 4.6s,5s,4.7s | 1s,1s,1s |

Using go 1.7.1 and g10k commit 7524778
Using ruby 2.1.5+deb8u2 and r10k v2.4.3
On Dell PowerEdge R320 Intel Xeon E5-2430 24 GB RAM on Debian Jessie

##### Benchmark w/o cache
```
rm -rf /tmp/g10k ; GDIR=$RANDOM ; mkdir /tmp/$GDIR/ ; cd /tmp/$GDIR/ ; \
wget https://raw.githubusercontent.com/xorpaul/g10k-environment/benchmark/Puppetfile ; \
time g10k -puppetfile

RDIR=$RANDOM ; mkdir /tmp/$RDIR/ ; cd /tmp/$RDIR/ ; \
wget https://raw.githubusercontent.com/xorpaul/g10k-environment/benchmark/Puppetfile ; \
time r10k puppetfile install
```
##### Benchmark w/ cache
```
cd /tmp/$GDIR/ ; time g10k -puppetfile
cd /tmp/$RDIR/ ; time r10k puppetfile install
```


# installation

Just grep the most recent stable release here:
https://github.com/xorpaul/g10k/releases


## Usage Docs
```
Usage of ./g10k:
  -branch string
        which git branch of the Puppet environment to update, e.g. core_foobar
  -cachedir string
        allows overriding of the g10k config file cachedir setting, the folder in which g10k will download git repositories and Forge modules
  -check4update
        only check if the is newer version of the Puppet module avaialable. Does implicitly set dryrun to true
  -checksum
        get the md5 check sum for each Puppetlabs Forge module and verify the integrity of the downloaded archive. Increases g10k run time!
  -config string
        which config file to use
  -debug
        log debug output, defaults to false
  -dryrun
        do not modify anything, just print what would be changed
  -force
        purge the Puppet environment directory and do a full sync
  -info
        log info output, defaults to false
  -maxworker int
        how many Goroutines are allowed to run in parallel for Git and Forge module resolving (default 50)
  -moduledir string
        allows overriding of Puppetfile specific moduledir setting, the folder in which Puppet modules will be extracted
  -puppetfile
        install all modules from Puppetfile in cwd
  -quiet
        no output, defaults to false
  -usemove
        do not use hardlinks to populate your Puppet environments with Puppetlabs Forge modules. Instead uses simple move commands and purges the Forge cache directory after each run! Var(&Useful for g10k runs inside a Docker container)
  -verbose
        log verbose output, defaults to false
  -version
        show build time and version number
```

Regarding anything usage/workflow you really can just use the great [puppetlabs/r10k](https://github.com/puppetlabs/r10k/blob/master/doc/dynamic-environments.mkd) docs as the [Puppetfile](https://github.com/puppetlabs/r10k/blob/master/doc/puppetfile.mkd) etc. are all intentionally kept unchanged. 
  

# additional Puppetfile features

- link Git module branch to the current environment branch:
```
mod 'awesomemodule',
    :git => 'http://github.com/foo/bar.git',
    :link => 'true'
```
If you are in environment branch `dev` then g10k would try to check out this module with branch `dev`.
This helps to be able to use the same Puppetfile over multiple environment branches and makes merges easier.
See https://github.com/xorpaul/g10k/issues/6 for details.


- only clone if branch/tag/commit exists
```
mod 'awesomemodule',
    :git => 'http://github.com/foo/bar.git',
    :ignore-unreachable => 'true'
```

In combination with the previous link feature you don't need to keep all environment branches also available for your modules.
See https://github.com/xorpaul/g10k/issues/9 for details.

- use different Forge base URL for your modules in your Puppetfile
```
forge.baseUrl http://foobar.domain.tld/
```
- skip version checks for latest Forge modules for a certain time to speed up the sync
```
forge.cacheTtl 4h
```
You need to specify the TTL value in the form of golang Duration (https://golang.org/pkg/time/#ParseDuration)

- try multiple Git branches for a Puppet module until one can be used
```
mod 'stdlib',
    :git => 'https://github.com/puppetlabs/puppetlabs-stdlib.git',
    :fallback => '4.889.x|foobar|master'
```

In this example g10k tries to use the branches:

`4.889.x` -> `foobar` -> `master`

Because there are no branches `4.889.x` or `foobar`.

All without failing or error messages.

Tip: You can see which branch was used, when using the `-verbose` parameter:

```
./g10k -puppetfile -verbose
2016/11/08 14:16:40 Executing git --git-dir ./tmp/https-__github.com_puppetlabs_puppetlabs-stdlib.git remote update --prune took 1.05001s
2016/11/08 14:16:40 Executing git --git-dir ./tmp/https-__github.com_puppetlabs_puppetlabs-stdlib.git log -n1 --pretty=format:%H master took 0.00299s
Synced ./Puppetfile with 4 git repositories and 0 Forge modules in 1.1s with git (1.1s sync, I/O 0.0s) and Forge (0.0s query+download, I/O 0.0s)
```

- additional Forge attribute `:sha256sum`:

For (some) increased security you can add a SHA256 sum for each Forge module, which g10k will verify after downloading the respective .tar.gz file:

```
mod 'puppetlabs/ntp', '6.0.0', :sha256sum => 'a988a172a3edde6ac2a26d0e893faa88d37bc47465afc50d55225a036906c944'

```

This does provide a very crude way to detect manipulated Forge modules and MITM attacks until the Puppetlabs Forge does support some sort of signing of Forge module releases.

If the SHA256 sum does not match the expected hash sum, g10k will warn the user and retry a download until giving up:

```
Resolving Forge modules (0/1)   --- [--------------------------------------------------------------------]   0%
WARNING: calculated sha256sum a988a172a3edde6ac2a26d0e893faa88d37bc47465afc50d55225a036906c944 for ./tmp/puppetlabs-ntp-6.0.0.tar.gz does not match expected sha256sum a988a172a3edde6ac2a26d0e893faa88d37bc47465afc50d55225a036906c94
Resolving Forge modules (0/1)   --- [--------------------------------------------------------------------]   0%
WARNING: calculated sha256sum a988a172a3edde6ac2a26d0e893faa88d37bc47465afc50d55225a036906c944 for ./tmp/puppetlabs-ntp-6.0.0.tar.gz does not match expected sha256sum a988a172a3edde6ac2a26d0e893faa88d37bc47465afc50d55225a036906c94
2016/12/08 18:05:11 downloadForgeModule(): giving up for Puppet module puppetlabs-ntp version: 6.0.0

```

(The Forge module retry count in case the Puppetlabs Forge provided MD5 sum, file archive size or SHA256 sum doesn't match defaults to `1`, but will be user configurable later.)


# additional g10k config features compared to r10k
- you can enforce version numbers of Forge modules in your Puppetfiles instead of `:latest` or `:present` by adding `force_forge_versions: true` to the g10k config in the specific resource

```
---
:cachedir: '/tmp/g10k'

sources:
  example:
    remote: 'https://github.com/xorpaul/g10k-environment.git'
    basedir: '/tmp/example/'
    force_forge_versions: true
```

If g10k then encounters `:latest` or `:present` for a Forge module it errors out with:

```
2016/11/15 18:45:38 Error: Found present setting for forge module in /tmp/example/example_benchmark/Puppetfile for module puppetlabs/concat line: mod 'puppetlabs/concat' and force_forge_versions is set to true! Please specify a version (e.g. '2.3.0')
```

- g10k can let you know if your source does not contain the branch you specified with the `-branch` parameter:

```
---
:cachedir: '/tmp/g10k'

sources:
  example:
    remote: 'https://github.com/xorpaul/g10k-environment.git'
    basedir: '/tmp/example/'
    warn_if_branch_is_missing: true
```

If you then call g10k with that config file and the following parameter `-branch nonExistingBranch`. You should get:

```
WARNING: Couldn't find specified branch 'nonExistingBranch' anywhere in source 'example' (https://github.com/xorpaul/g10k-environment.git)
```

This can be helpful if you use a dedicated hiera repository/g10k source and you want to ensure that you always have a matching branch, see [#45](https://github.com/xorpaul/g10k/issues/45)

- By default g10k fails if one of your Puppet environments could not be completely populated (e.g. if one of your Puppet Git module branches doesn't exist anymore). You can change this by setting `ignore_unreachable_modules` to true in your g10k config:

```
---
:cachedir: '/tmp/g10k'
ignore_unreachable_modules: true

sources:
  example:
    remote: 'https://github.com/xorpaul/g10k-failing-env.git'
    basedir: '/tmp/failing/'
```

If you then call g10k with that config file and at least the `info` verbosity level, you should get: 

```
Failed to populate module /tmp/failing/master/modules//sensu/ but ignore-unreachable is set. Continuing...
```

See [#57](https://github.com/xorpaul/g10k/issues/57) for details.

# building
```
# only initially needed to resolve all dependencies
go get
# actually compiling the binary with the current date as build time
BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S') && go build -ldflags "-s -w -X main.buildtime=$BUILDTIME"
```

# execute example with debug output
```
./g10k -debug -config test.yaml
```
