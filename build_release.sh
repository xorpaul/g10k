#! /usr/bin/env bash
#set -e

if [ $# -ne 2 ]; then
  echo "need the version number and release comment as argument"
  echo "e.g. ${0} 0.4.5 'fix local modules and modules with install_path purging bug #80 #82'"
  echo "Aborting..."
	exit 1
fi

time go test -v

if [ $? -ne 0 ]; then 
  echo "Tests unsuccessfull"
  echo "Aborting..."
	exit 1
fi

# try to get the project name from the current working directory
projectname=${PWD##*/}

#sed -i "s/${projectname} version [^ ]*/${projectname} version ${1}/" ${projectname}.go
#git add ${projectname}.go
#git commit -m "bump version to v${1}"

echo "creating git tag v${1}"
git tag v${1}
echo "pushing git tag v${1}"
git push -f --tags
git push

echo "creating github release v${1}"
github-release release  --user xorpaul     --repo ${projectname}     --tag v${1}     --name "v${1}"     --description "${2}"

upx=`which upx`

if [ ${#upx} -gt 0 ]; then
  echo "building and uploading ${projectname}-darwin-amd64-debug"
  BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S') BUILDVERSION=$(git describe --tags) && env GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.buildtime=${BUILDTIME} -X main.buildversion=${BUILDVERSION}" && date
  zip ${projectname}-darwin-amd64-debug.zip ${projectname}
  github-release upload     --user xorpaul     --repo ${projectname}     --tag v${1}     --name "${projectname}-darwin-amd64-debug.zip" --file ${projectname}-darwin-amd64-debug.zip
fi

echo "building and uploading ${projectname}-darwin-amd64"
BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S') BUILDVERSION=$(git describe --tags) && env GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X main.buildtime=${BUILDTIME} -X main.buildversion=${BUILDVERSION}" && date
if [ ${#upx} -gt 0 ]; then
  $upx --brute ${projectname}
fi
zip ${projectname}-darwin-amd64.zip ${projectname}
github-release upload     --user xorpaul     --repo ${projectname}     --tag v${1}     --name "${projectname}-darwin-amd64.zip" --file ${projectname}-darwin-amd64.zip


if [ ${#upx} -gt 0 ]; then
  echo "building and uploading ${projectname}-darwin-arm64-debug"
  BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S') BUILDVERSION=$(git describe --tags) && env GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.buildtime=${BUILDTIME} -X main.buildversion=${BUILDVERSION}" && date
  zip ${projectname}-darwin-arm64-debug.zip ${projectname}
  github-release upload     --user xorpaul     --repo ${projectname}     --tag v${1}     --name "${projectname}-darwin-arm64-debug.zip" --file ${projectname}-darwin-arm64-debug.zip
fi

echo "building and uploading ${projectname}-darwin-arm64"
BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S') BUILDVERSION=$(git describe --tags) && env GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w -X main.buildtime=${BUILDTIME} -X main.buildversion=${BUILDVERSION}" && date
if [ ${#upx} -gt 0 ]; then
  $upx --brute ${projectname}
fi
zip ${projectname}-darwin-arm64.zip ${projectname}
github-release upload     --user xorpaul     --repo ${projectname}     --tag v${1}     --name "${projectname}-darwin-arm64.zip" --file ${projectname}-darwin-arm64.zip




if [ ${#upx} -gt 0 ]; then
  echo "building and uploading ${projectname}-linux-amd64-debug"
  BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S') BUILDVERSION=$(git describe --tags) && go build -race -ldflags "-X main.buildtime=${BUILDTIME} -X main.buildversion=${BUILDVERSION}" && date && env ${projectname}_cachedir=/tmp/${projectname} ./${projectname} -config test.yaml -branch benchmark 2>&1
  zip ${projectname}-linux-amd64-debug.zip ${projectname}
  github-release upload     --user xorpaul     --repo ${projectname}     --tag v${1}     --name "${projectname}-linux-amd64-debug.zip" --file ${projectname}-linux-amd64-debug.zip
fi

echo "building and uploading ${projectname}-linux-amd64"
BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S') BUILDVERSION=$(git describe --tags) && go build -race -ldflags "-s -w -X main.buildtime=${BUILDTIME} -X main.buildversion=${BUILDVERSION}" && date && env ${projectname}_cachedir=/tmp/${projectname} ./${projectname} -config test.yaml -branch benchmark 2>&1
if [ ${#upx} -gt 0 ]; then
  $upx --brute ${projectname}
fi
zip ${projectname}-linux-amd64.zip ${projectname}
github-release upload     --user xorpaul     --repo ${projectname}     --tag v${1}     --name "${projectname}-linux-amd64.zip" --file ${projectname}-linux-amd64.zip

