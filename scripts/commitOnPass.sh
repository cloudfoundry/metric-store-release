#!/bin/bash -e

if [[ $1 == "[PASS]" ]]; then
  if [[ $(git status -s) == "" ]]; then
    echo "No changes"
    exit 0
  fi

  shift

  git add .
  git commit -m "[Autocommit] $*"
else
  echo "No commit"
fi
