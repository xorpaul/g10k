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

updated 2016-10-14

Using Puppetfile with 4 git repositories and 25 Forge modules
https://github.com/xorpaul/g10k-environment/blob/benchmark/Puppetfile

||w/o cache| w/ cache
|------------|------------ | -------------
|r10k|1m14s,1m18s,1m12s|18s,17s,17s
|g10k|4.6s,5s,4.7s|1s,1s,1s

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




## Usage Docs
```
Usage of ./g10k:
  -branch string
    	which git branch of the Puppet environment to update, e.g. core_foobar
  -check4update
    	only check if the is newer version of the Puppet module avaialable. Does implicitly set parameter dryrun to true
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
  -puppetfile
    	install all modules from Puppetfile in cwd
  -usemove
    	do not use hardlinks to populate your Puppet environments with Puppetlabs Forge modules. Uses simple move instead of hard links and purge the Forge cache directory after each run!
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
