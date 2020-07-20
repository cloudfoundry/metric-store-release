#!/bin/bash

source $(dirname $0)/fly-login.sh
PIPELINE=${PIPELINE:-acceptance}
JOB=${JOB:-unit-tests}

( for build in $(fly -t "$FLY_TARGET" builds -j "$PIPELINE/$JOB" | awk '/succeeded/ {print $3}'); do
    fly -t "$FLY_TARGET" watch -j "$PIPELINE/$JOB" -b "$build" | grep '\[Fail\]'
done ) | sort | uniq -c | sort -rn
