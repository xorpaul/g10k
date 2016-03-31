[![Build Status](https://travis-ci.org/xorpaul/g10k.svg?branch=master)](https://travis-ci.org/xorpaul/g10k) [![Go Report Card](https://goreportcard.com/badge/github.com/xorpaul/g10k)](https://goreportcard.com/report/github.com/xorpaul/g10k)
# g10k
My r10k fork written in Go, designed to work as a drop-in replacement* in place of [puppetlabs/r10k](https://github.com/puppetlabs/r10k).

### Why fork?
  - Lack of caching/version-pre-checking in current r10k implementation hurt perfomance beyond a certain # of Modules per Puppetfile
  - We need distinct SSHKeys for each source in the r10k.yaml and 'rugged' never really wanted to play nice (fixed in r10k [2.2.0](https://github.com/puppetlabs/r10k/blob/master/CHANGELOG.mkd#220 ))
  - Good excuse to try Go ;)

### Changes breaking 'true' drop-in replacement capability
  - No SVN support
  - No 'local'-Modules support

### Non-breaking changes to r10k
  - Download/Cache each git Puppet Module repository and each Puppetlabs Forge Puppet Module for each respective Version only once
  - Most things (git, forge, and copy operations) done in parallel over each branch
  - Optional support for different ssh keys for each source inside the r10k.yaml

### Pseudo "benchmark"

Using Puppetfile with 4 git repositories and 25 Forge modules

||w/o cache| w/ cache
|------------|------------ | -------------
|r10k|4m11s,3m44s,3m54s|12s,15s,12s
|g10k|6s,6s,6s|1s,2s,1s

https://github.com/xorpaul/g10k-environment/blob/benchmark/Puppetfile

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
Regarding anything usage/workflow you really can just use the great [puppetlabs/r10k](https://github.com/puppetlabs/r10k/blob/master/doc/dynamic-environments.mkd) docs as the [Puppetfile](https://github.com/puppetlabs/r10k/blob/master/doc/puppetfile.mkd) etc. are all intentionally kept unchanged. 
  
# building
```
# only initially need to resolve all dependecies
go get
# actually compiling the binary with the current date as build time
BUILDTIME=$(date -u '+%Y-%m-%d %H:%M:%S') ; go build -ldflags "-X main.buildtime '$BUILDTIME'"
```

# testing
```
./g10k -debug -config test.yaml
```
