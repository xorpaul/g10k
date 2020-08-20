#! /bin/bash

if [ ${#} -lt 2 ]; then
  echo "${0} expects at least the current branch name and the regex as parameters"
  echo "example: ${0} master ^(single|master)$"
  exit 1
fi

if [[ ${1} =~ ${2} ]]; then
  exit 0
else
  exit 1
fi
