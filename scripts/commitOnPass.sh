#!/bin/bash -e

if [[ $1 == "[PASS]" ]]; then
  if [[ $(git status -s) == "" ]]; then
    echo "No changes"
    exit 0
  fi

  git add .
  git commit -m "[Autocommit] Tests Passed"
else
  echo "No commit"
fi