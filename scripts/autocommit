#!/bin/bash -e

function squashHead() {
  branch=$(git branch --show-current)
  prevCommit=$(git log --oneline "${branch}" | grep -v '\[Autocommit\]' | head -1 | awk '{print $1}')

  echo ${prevCommit}
}

function printUsage() {
  echo "Run tests and autocommit"
  echo "usage: $0 path/to/package/to/test"
  echo "       Watches that package for changes and automatically runs tests on changes"
  echo "       Automatically commits when tests pass with [Autocommit] tag"
  echo ""
  echo "Squash autocommits"
  echo "usage: $0 squash \"commit message\""
  echo "       Squashes last concurrent [Autocommit] tagged commits into a single commit"
  echo ""
  echo "Diff squash"
  echo "usage: $0 diff"
  echo "       show the diff of all that you are about to squash but don't squash"
  echo ""
  echo "Unroll commits"
  echo "usage: $0 unroll"
  echo "       unroll your autocommits but do not automatically commit the squash"

}

if [[ $# -lt 1 || $1 == "-h" ]]; then
  printUsage
  exit 1
fi

scriptdir=$(dirname "$0")
projectdir=$(git rev-parse --show-toplevel)
testdir="${projectdir}/$1"

if [[ $1 == "squash" ]]; then
  git reset "$(squashHead)"
  git add -A
  git commit -m "$2"
elif [[ $1 == "unroll" ]]; then
  git reset "$(squashHead)"
  git add -A
  git commit -m "$2"
elif [[ $1 == "diff" ]]; then
  git diff "$(squashHead)"
else
  if [ ! -d "${testdir}" ]; then
    echo "path ${testdir} does not exist"
    exit 1
  fi

  ginkgo watch --succinct --afterSuiteHook="${scriptdir}/commitOnPass.sh (ginkgo-suite-passed)" \
    "${testdir}"
fi
