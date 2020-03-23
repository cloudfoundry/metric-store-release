#!/usr/bin/env bash

scriptdir=$(dirname "$0")
projectdir="${scriptdir}/../"

if [[ $1 == "squash" ]]; then
  branch=$(git branch --show-current)
  prevCommit=$(git log --oneline ${branch} | grep -v '\[Autocommit\]' | head -1 | awk '{print $1}')
  git reset $prevCommit
  git add -A
  git commit -m "$2"
else
  ginkgo watch --succinct --afterSuiteHook="${scriptdir}/commitOnPass.sh (ginkgo-suite-passed)" \
    ${projectdir}$1
fi