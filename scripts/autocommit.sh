#!/bin/bash -e

if [[ $# -lt 1 ]]; then
  echo "Run tests and autocommit"
  echo "usage: $0 path/to/package/to/test"
  echo "       Watches that package for changes and automatically runs tests on changes"
  echo "       Automatically commits when tests pass with [Autocommit] tag"
  echo ""
  echo "Squash autocommits"
  echo "usage: $0 squash \"commit message\""
  echo "       Squashes last concurrent [Autocommit] tagged commits into a single commit"
  exit 1
fi

scriptdir=$(dirname "$0")
projectdir=$(git rev-parse --show-toplevel)
testdir="${projectdir}/$1"

if [[ $1 == "squash" ]]; then
  branch=$(git branch --show-current)
  prevCommit=$(git log --oneline "${branch}" | grep -v '\[Autocommit\]' | head -1 | awk '{print $1}')

  git reset "$prevCommit"
  git add -A
  git commit -m "$2"
else
  if [ ! -d "${testdir}" ]; then
    echo "path ${testdir} does not exist"
    exit 1
  fi

  ginkgo watch --succinct --afterSuiteHook="${scriptdir}/commitOnPass.sh (ginkgo-suite-passed)" \
    "${testdir}"
fi