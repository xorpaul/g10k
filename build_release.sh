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

sed -i "s/g10k version [^ ]*/g10k version ${1}/" g10k.go
git add g10k.go
git commit -m "bump version to v${1}"

echo "creating git tag v${1}"
git tag v${1}
echo "pushing git tag v${1}"
git push -f --tags
git push

echo "creating github release v${1}"
github-release release  --user xorpaul     --repo g10k     --tag v${1}     --name "v${1}"     --description "${2}"

echo "building and uploading g10k-darwin-amd64"
BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S') && env GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X main.buildtime=$BUILDTIME" && date
zip g10k-darwin-amd64.zip g10k
github-release upload     --user xorpaul     --repo g10k     --tag v${1}     --name "g10k-darwin-amd64.zip" --file g10k-darwin-amd64.zip

echo "building and uploading g10k-linux-amd64"
BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S') && go build -race -ldflags "-s -w -X main.buildtime=$BUILDTIME" && date && env g10k_cachedir=./tmp ./g10k -config test.yaml -branch benchmark 2>&1 -verbose
zip g10k-linux-amd64.zip g10k
github-release upload     --user xorpaul     --repo g10k     --tag v${1}     --name "g10k-linux-amd64.zip" --file g10k-linux-amd64.zip

