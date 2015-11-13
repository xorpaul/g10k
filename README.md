# g10k
My r10k fork written in Go, designed to work as a drop-in replacement* in place of [puppetlabs/r10k](https://github.com/puppetlabs/r10k).

### Why fork?
  - Lack of caching/version-pre-checking in current r10k implementation hurt perfomance beyond a certain # of Modules per Puppetfile
  - We need distinct SSHKeys for each source in the r10k.yaml and 'rugged' never really wanted to play nice
  - Good excuse to try Go ;)

### Changes breaking 'true' drop-in replacement capability
  - No SVN support
  - No 'local'-Modules support

### Non-breaking changes to r10k
  - Download/Cache each git Puppet Module repository and each Puppetlabs Forge Puppet Module for each respective Version only once
  - Most things (git, forge, and copy operations) done in parallel over each branch
  - Optional support for different ssh keys for each source inside the r10k.yaml

## Usage Docs
Regarding anything usage/workflow you really can just use the great [puppetlabs/r10k](https://github.com/puppetlabs/r10k/blob/master/doc/dynamic-environments.mkd) docs as the [Puppetfile](https://github.com/puppetlabs/r10k/blob/master/doc/puppetfile.mkd) etc. are all intentionally kept unchanged. 
  
# building
```
BUILDTIME=$(date -u '+%Y-%m-%d %H:%M:%S') ; go build -ldflags "-X main.buildtime '$BUILDTIME'"
```

# testing
```
./g10k -debug -config test.yaml
```

## example output without cache
```
2015/08/11 18:07:45 DEBUG Using as config file: test.yaml
2015/08/11 18:07:45 checkDirAndCreate(): trying to create dir '/tmp/g10k'
2015/08/11 18:07:45 DEBUG Using as cachedir: /tmp/g10k/
2015/08/11 18:07:45 checkDirAndCreate(): trying to create dir '/tmp/g10k/forge/'
2015/08/11 18:07:45 DEBUG Using as cachedir/forge: /tmp/g10k/forge/
2015/08/11 18:07:45 checkDirAndCreate(): trying to create dir '/tmp/g10k/modules/'
2015/08/11 18:07:45 DEBUG Using as cachedir/modules: /tmp/g10k/modules/
2015/08/11 18:07:45 checkDirAndCreate(): trying to create dir '/tmp/g10k/environments/'
2015/08/11 18:07:45 DEBUG Using as cachedir/environments: /tmp/g10k/environments/
2015/08/11 18:07:45 checkDirAndCreate(): trying to create dir '/tmp/example'
2015/08/11 18:07:45 DEBUG Using as basedir for sourceexample: /tmp/example/
2015/08/11 18:07:45 DEBUG Puppet environment: example (remote=https://github.com/xorpaul/g10k-environment.git, basedir=/tmp/example/, private_key=, prefix=true)
2015/08/11 18:07:45 DEBUG Using as basedir: /tmp/example/
2015/08/11 18:07:45 DEBUG Executing git clone --mirror https://github.com/xorpaul/g10k-environment.git /tmp/g10k/environments/example.git
2015/08/11 18:07:46 Executing git clone --mirror https://github.com/xorpaul/g10k-environment.git /tmp/g10k/environments/example.git took 1.26937s
2015/08/11 18:07:46 DEBUG Executing git --git-dir /tmp/g10k/environments/example.git for-each-ref --sort=-committerdate --format=%(refname:short)
2015/08/11 18:07:46 Executing git --git-dir /tmp/g10k/environments/example.git for-each-ref --sort=-committerdate --format=%(refname:short) took 0.00215s
2015/08/11 18:07:46 DEBUG Resolving branch: dev
2015/08/11 18:07:46 DEBUG Resolving branch: master
2015/08/11 18:07:46 DEBUG createOrPurgeDir(): trying to create dir: /tmp/example/example_dev/
2015/08/11 18:07:46 DEBUG createOrPurgeDir(): trying to create dir: /tmp/example/example_master/
2015/08/11 18:07:46 syncToModuleDir(): Executing git --git-dir /tmp/g10k/environments/example.git archive dev | tar -x -C /tmp/example/example_dev/ took 0.00747s
2015/08/11 18:07:46 DEBUG readPuppetfile(): Trying to parse: /tmp/example/example_dev/Puppetfile
2015/08/11 18:07:46 syncToModuleDir(): Executing git --git-dir /tmp/g10k/environments/example.git archive master | tar -x -C /tmp/example/example_master/ took 0.00837s
2015/08/11 18:07:46 DEBUG readPuppetfile(): Trying to parse: /tmp/example/example_master/Puppetfile
2015/08/11 18:07:46 DEBUG Resolving example_dev
2015/08/11 18:07:46 DEBUG Resolving example_master
2015/08/11 18:07:46 DEBUG git repo url https://github.com/puppetlabs/puppetlabs-apt.git with ssh key 
2015/08/11 18:07:46 DEBUG Executing git clone --mirror https://github.com/puppetlabs/puppetlabs-apt.git /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git
2015/08/11 18:07:49 Executing git clone --mirror https://github.com/puppetlabs/puppetlabs-apt.git /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git took 2.52431s
2015/08/11 18:07:49 DEBUG Trying to get forge module puppetlabs-ntp-3.3.0
2015/08/11 18:07:49 DEBUG Trying to get forge module camptocamp-postfix-1.1.0
2015/08/11 18:07:49 DEBUG Trying to get forge module puppetlabs-concat-latest
2015/08/11 18:07:49 DEBUG Trying to get forge module camptocamp-postfix-1.2.2
2015/08/11 18:07:49 DEBUG Trying to get forge module puppetlabs-inifile-latest
2015/08/11 18:07:49 DEBUG createOrPurgeDir(): trying to create dir: /tmp/g10k/forge/camptocamp-postfix-1.1.0
2015/08/11 18:07:49 DEBUG doModuleInstallOrNothing(): /tmp/g10k/forge/puppetlabs-inifile-latest did not exists, fetching module
2015/08/11 18:07:49 DEBUG doModuleInstallOrNothing(): /tmp/g10k/forge/puppetlabs-concat-latest did not exists, fetching module
2015/08/11 18:07:49 DEBUG Trying to get forge module puppetlabs-apt-1.8.0
2015/08/11 18:07:49 DEBUG createOrPurgeDir(): trying to create dir: /tmp/g10k/forge/camptocamp-postfix-1.2.2
2015/08/11 18:07:49 DEBUG createOrPurgeDir(): trying to create dir: /tmp/g10k/forge/puppetlabs-apt-1.8.0
2015/08/11 18:07:49 DEBUG Trying to get forge module puppetlabs-stdlib-4.6.0
2015/08/11 18:07:49 DEBUG createOrPurgeDir(): trying to create dir: /tmp/g10k/forge/puppetlabs-stdlib-4.6.0
2015/08/11 18:07:49 DEBUG Trying to get forge module puppetlabs-stdlib-4.7.0
2015/08/11 18:07:49 DEBUG createOrPurgeDir(): trying to create dir: /tmp/g10k/forge/puppetlabs-ntp-3.3.0
2015/08/11 18:07:49 DEBUG createOrPurgeDir(): trying to create dir: /tmp/g10k/forge/puppetlabs-stdlib-4.7.0
2015/08/11 18:07:49 Querying Forge API https://forgeapi.puppetlabs.com:443/v3/modules?query=puppetlabs-concat took 0.57786s
2015/08/11 18:07:49 DEBUG queryForgeApi(): found current version 1.2.4
2015/08/11 18:07:50 Querying Forge API https://forgeapi.puppetlabs.com:443/v3/modules?query=puppetlabs-inifile took 0.62577s
2015/08/11 18:07:50 DEBUG queryForgeApi(): found current version 1.4.1
2015/08/11 18:07:50 GETing https://forgeapi.puppetlabs.com/v3/files/puppetlabs-stdlib-4.7.0.tar.gz took 0.80419s
2015/08/11 18:07:50 DEBUG downloadForgeModule(): Trying to create /tmp/g10k/forge/puppetlabs-stdlib-4.7.0.tar.gz
2015/08/11 18:07:50 GETing https://forgeapi.puppetlabs.com/v3/files/camptocamp-postfix-1.1.0.tar.gz took 0.84816s
2015/08/11 18:07:50 DEBUG downloadForgeModule(): Trying to create /tmp/g10k/forge/camptocamp-postfix-1.1.0.tar.gz
2015/08/11 18:07:50 GETing https://forgeapi.puppetlabs.com/v3/files/puppetlabs-apt-1.8.0.tar.gz took 0.99611s
2015/08/11 18:07:50 DEBUG downloadForgeModule(): Trying to create /tmp/g10k/forge/puppetlabs-apt-1.8.0.tar.gz
2015/08/11 18:07:50 GETing https://forgeapi.puppetlabs.com/v3/files/puppetlabs-stdlib-4.6.0.tar.gz took 1.00957s
2015/08/11 18:07:50 DEBUG downloadForgeModule(): Trying to create /tmp/g10k/forge/puppetlabs-stdlib-4.6.0.tar.gz
2015/08/11 18:07:50 GETing https://forgeapi.puppetlabs.com/v3/files/camptocamp-postfix-1.2.2.tar.gz took 1.02098s
2015/08/11 18:07:50 DEBUG downloadForgeModule(): Trying to create /tmp/g10k/forge/camptocamp-postfix-1.2.2.tar.gz
2015/08/11 18:07:50 GETing https://forgeapi.puppetlabs.com/v3/files/puppetlabs-ntp-3.3.0.tar.gz took 1.02499s
2015/08/11 18:07:50 DEBUG downloadForgeModule(): Trying to create /tmp/g10k/forge/puppetlabs-ntp-3.3.0.tar.gz
2015/08/11 18:07:50 GETing https://forgeapi.puppetlabs.com/v3/files/puppetlabs-concat-1.2.4.tar.gz took 0.88163s
2015/08/11 18:07:50 DEBUG downloadForgeModule(): Trying to create /tmp/g10k/forge/puppetlabs-concat-1.2.4.tar.gz
2015/08/11 18:07:51 GETing https://forgeapi.puppetlabs.com/v3/files/puppetlabs-inifile-1.4.1.tar.gz took 1.02262s
2015/08/11 18:07:51 DEBUG downloadForgeModule(): Trying to create /tmp/g10k/forge/puppetlabs-inifile-1.4.1.tar.gz
2015/08/11 18:07:51 DEBUG Syncing example_master
2015/08/11 18:07:51 DEBUG Using as basedir for sourceexample: /tmp/example/
2015/08/11 18:07:51 DEBUG createOrPurgeDir(): trying to create dir: /tmp/example/example_master/external_modules
2015/08/11 18:07:51 DEBUG createOrPurgeDir(): trying to create dir: /tmp/example/example_master/external_modules/apt
2015/08/11 18:07:51 syncToModuleDir(): Executing git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git archive 3e64758ca720d5325d40e11bb8619675b6c0c75f | tar -x -C /tmp/example/example_master/external_modules/apt took 0.01160s
2015/08/11 18:07:51 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-inifile-latest/ /tmp/example/example_master/external_modules/inifile took 0.00569s
2015/08/11 18:07:51 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-ntp-3.3.0/ /tmp/example/example_master/external_modules/ntp took 0.00557s
2015/08/11 18:07:51 Executing cp --link --archive /tmp/g10k/forge/camptocamp-postfix-1.2.2/ /tmp/example/example_master/external_modules/postfix took 0.00426s
2015/08/11 18:07:51 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-stdlib-4.6.0/ /tmp/example/example_master/external_modules/stdlib took 0.00824s
2015/08/11 18:07:51 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-concat-latest/ /tmp/example/example_master/external_modules/concat took 0.00538s
2015/08/11 18:07:51 DEBUG Syncing example_dev
2015/08/11 18:07:51 DEBUG Using as basedir for sourceexample: /tmp/example/
2015/08/11 18:07:51 DEBUG createOrPurgeDir(): trying to create dir: /tmp/example/example_dev/external_modules
2015/08/11 18:07:51 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-apt-1.8.0/ /tmp/example/example_dev/external_modules/apt took 0.00581s
2015/08/11 18:07:51 Executing cp --link --archive /tmp/g10k/forge/camptocamp-postfix-1.1.0/ /tmp/example/example_dev/external_modules/postfix took 0.00526s
2015/08/11 18:07:51 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-stdlib-4.7.0/ /tmp/example/example_dev/external_modules/stdlib took 0.00942s
2015/08/11 18:07:51 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-concat-latest/ /tmp/example/example_dev/external_modules/concat took 0.00469s
2015/08/11 18:07:51 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-inifile-latest/ /tmp/example/example_dev/external_modules/inifile took 0.00694s
Synced test.yaml : 3 git repositories and 10 Forge modules in 5.5 s with git sync time of 3.7958273289999998 s and Forge query + download in 8.811895958000001 s done in 4 threads parallel
```

## example output with cache
```
2015/08/11 18:08:46 DEBUG Using as config file: test.yaml
2015/08/11 18:08:46 DEBUG Using as cachedir: /tmp/g10k/
2015/08/11 18:08:46 DEBUG Using as cachedir/forge: /tmp/g10k/forge/
2015/08/11 18:08:46 DEBUG Using as cachedir/modules: /tmp/g10k/modules/
2015/08/11 18:08:46 DEBUG Using as cachedir/environments: /tmp/g10k/environments/
2015/08/11 18:08:46 DEBUG Using as basedir for sourceexample: /tmp/example/
2015/08/11 18:08:46 DEBUG Puppet environment: example (remote=https://github.com/xorpaul/g10k-environment.git, basedir=/tmp/example/, private_key=, prefix=true)
2015/08/11 18:08:46 DEBUG Using as basedir: /tmp/example/
2015/08/11 18:08:46 DEBUG Executing git --git-dir /tmp/g10k/environments/example.git remote update
2015/08/11 18:08:46 Executing git --git-dir /tmp/g10k/environments/example.git remote update took 0.71722s
2015/08/11 18:08:46 DEBUG Executing git --git-dir /tmp/g10k/environments/example.git for-each-ref --sort=-committerdate --format=%(refname:short)
2015/08/11 18:08:46 Executing git --git-dir /tmp/g10k/environments/example.git for-each-ref --sort=-committerdate --format=%(refname:short) took 0.00227s
2015/08/11 18:08:46 DEBUG Resolving branch: dev
2015/08/11 18:08:46 DEBUG Resolving branch: master
2015/08/11 18:08:46 DEBUG createOrPurgeDir(): Trying to remove: /tmp/example/example_dev/
2015/08/11 18:08:46 DEBUG createOrPurgeDir(): Trying to remove: /tmp/example/example_master/
2015/08/11 18:08:46 DEBUG createOrPurgeDir(): trying to create dir: /tmp/example/example_dev/
2015/08/11 18:08:46 DEBUG createOrPurgeDir(): trying to create dir: /tmp/example/example_master/
2015/08/11 18:08:46 syncToModuleDir(): Executing git --git-dir /tmp/g10k/environments/example.git archive dev | tar -x -C /tmp/example/example_dev/ took 0.00464s
2015/08/11 18:08:46 DEBUG readPuppetfile(): Trying to parse: /tmp/example/example_dev/Puppetfile
2015/08/11 18:08:47 syncToModuleDir(): Executing git --git-dir /tmp/g10k/environments/example.git archive master | tar -x -C /tmp/example/example_master/ took 0.00459s
2015/08/11 18:08:47 DEBUG readPuppetfile(): Trying to parse: /tmp/example/example_master/Puppetfile
2015/08/11 18:08:47 DEBUG Resolving example_dev
2015/08/11 18:08:47 DEBUG Resolving example_master
2015/08/11 18:08:47 DEBUG git repo url https://github.com/puppetlabs/puppetlabs-apt.git with ssh key 
2015/08/11 18:08:47 DEBUG Executing git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git remote update
2015/08/11 18:08:48 Executing git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git remote update took 0.99846s
2015/08/11 18:08:48 DEBUG Trying to get forge module puppetlabs-concat-latest
2015/08/11 18:08:48 DEBUG Trying to get forge module puppetlabs-ntp-3.3.0
2015/08/11 18:08:48 DEBUG doModuleInstallOrNothing(): Using cache for puppetlabs-ntp in version 3.3.0 because /tmp/g10k/forge/puppetlabs-ntp-3.3.0 exists
2015/08/11 18:08:48 DEBUG Trying to get forge module puppetlabs-stdlib-4.6.0
2015/08/11 18:08:48 DEBUG Trying to get forge module puppetlabs-stdlib-4.7.0
2015/08/11 18:08:48 DEBUG Trying to get forge module puppetlabs-apt-1.8.0
2015/08/11 18:08:48 DEBUG doModuleInstallOrNothing(): Using cache for puppetlabs-stdlib in version 4.6.0 because /tmp/g10k/forge/puppetlabs-stdlib-4.6.0 exists
2015/08/11 18:08:48 DEBUG Trying to get forge module camptocamp-postfix-1.1.0
2015/08/11 18:08:48 DEBUG adding If-Modified-Since:Tue, 11 Aug 2015 18:07:50 GMT to Forge query
2015/08/11 18:08:48 DEBUG doModuleInstallOrNothing(): Using cache for camptocamp-postfix in version 1.1.0 because /tmp/g10k/forge/camptocamp-postfix-1.1.0 exists
2015/08/11 18:08:48 DEBUG Trying to get forge module puppetlabs-inifile-latest
2015/08/11 18:08:48 DEBUG doModuleInstallOrNothing(): Using cache for puppetlabs-apt in version 1.8.0 because /tmp/g10k/forge/puppetlabs-apt-1.8.0 exists
2015/08/11 18:08:48 DEBUG doModuleInstallOrNothing(): Using cache for puppetlabs-stdlib in version 4.7.0 because /tmp/g10k/forge/puppetlabs-stdlib-4.7.0 exists
2015/08/11 18:08:48 DEBUG Trying to get forge module camptocamp-postfix-1.2.2
2015/08/11 18:08:48 DEBUG doModuleInstallOrNothing(): Using cache for camptocamp-postfix in version 1.2.2 because /tmp/g10k/forge/camptocamp-postfix-1.2.2 exists
2015/08/11 18:08:48 DEBUG adding If-Modified-Since:Tue, 11 Aug 2015 18:07:51 GMT to Forge query
2015/08/11 18:08:48 Querying Forge API https://forgeapi.puppetlabs.com:443/v3/modules?query=puppetlabs-inifile took 0.55173s
2015/08/11 18:08:48 DEBUG doModuleInstallOrNothing(): Got 304 nothing to do for modulepuppetlabs-inifile
2015/08/11 18:08:48 DEBUG doModuleInstallOrNothing(): Using cache for puppetlabs-inifile in version latest because /tmp/g10k/forge/puppetlabs-inifile-latest exists
2015/08/11 18:08:48 Querying Forge API https://forgeapi.puppetlabs.com:443/v3/modules?query=puppetlabs-concat took 0.55330s
2015/08/11 18:08:48 DEBUG doModuleInstallOrNothing(): Got 304 nothing to do for modulepuppetlabs-concat
2015/08/11 18:08:48 DEBUG doModuleInstallOrNothing(): Using cache for puppetlabs-concat in version latest because /tmp/g10k/forge/puppetlabs-concat-latest exists
2015/08/11 18:08:48 DEBUG Syncing example_dev
2015/08/11 18:08:48 DEBUG Using as basedir for sourceexample: /tmp/example/
2015/08/11 18:08:48 DEBUG createOrPurgeDir(): trying to create dir: /tmp/example/example_dev/external_modules
2015/08/11 18:08:48 Executing cp --link --archive /tmp/g10k/forge/camptocamp-postfix-1.1.0/ /tmp/example/example_dev/external_modules/postfix took 0.00585s
2015/08/11 18:08:48 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-stdlib-4.7.0/ /tmp/example/example_dev/external_modules/stdlib took 0.00974s
2015/08/11 18:08:48 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-concat-latest/ /tmp/example/example_dev/external_modules/concat took 0.00558s
2015/08/11 18:08:48 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-inifile-latest/ /tmp/example/example_dev/external_modules/inifile took 0.00590s
2015/08/11 18:08:48 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-apt-1.8.0/ /tmp/example/example_dev/external_modules/apt took 0.00589s
2015/08/11 18:08:48 DEBUG Syncing example_master
2015/08/11 18:08:48 DEBUG Using as basedir for sourceexample: /tmp/example/
2015/08/11 18:08:48 DEBUG createOrPurgeDir(): trying to create dir: /tmp/example/example_master/external_modules
2015/08/11 18:08:48 DEBUG createOrPurgeDir(): trying to create dir: /tmp/example/example_master/external_modules/apt
2015/08/11 18:08:48 syncToModuleDir(): Executing git --git-dir /tmp/g10k/modules/https-__github.com_puppetlabs_puppetlabs-apt.git archive 3e64758ca720d5325d40e11bb8619675b6c0c75f | tar -x -C /tmp/example/example_master/external_modules/apt took 0.01334s
2015/08/11 18:08:48 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-concat-latest/ /tmp/example/example_master/external_modules/concat took 0.00500s
2015/08/11 18:08:48 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-inifile-latest/ /tmp/example/example_master/external_modules/inifile took 0.00446s
2015/08/11 18:08:48 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-ntp-3.3.0/ /tmp/example/example_master/external_modules/ntp took 0.00326s
2015/08/11 18:08:48 Executing cp --link --archive /tmp/g10k/forge/camptocamp-postfix-1.2.2/ /tmp/example/example_master/external_modules/postfix took 0.00319s
2015/08/11 18:08:48 Executing cp --link --archive /tmp/g10k/forge/puppetlabs-stdlib-4.6.0/ /tmp/example/example_master/external_modules/stdlib took 0.00977s
Synced test.yaml : 3 git repositories and 10 Forge modules in 2.4 s with git sync time of 1.7179520810000002 s and Forge query + download in 1.105038049 s done in 4 threads parallel
```
