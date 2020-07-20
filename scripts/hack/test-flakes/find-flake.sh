#!/bin/bash

function usage() {
    echo "$0 - find the builds that represent a test flake"
    echo ""
    echo "Usage: $0 <test-description>"
    echo ""
    echo "Params:"
    echo "    test-description    The first param to It() for the test you want to find"
    echo ""
    echo "ENV:"
    echo "    PIPELINE            Name of the pipeline containing the job that runs the tests (optional)"
    echo "    JOB                 Name of the job that runs the tests (optional)"
    echo ""

    exit 127
}

function main() {
    if [[ $# -lt 1 ]]; then
        usage
    fi

    source $(dirname $0)/fly-login.sh
    PIPELINE=${PIPELINE:-acceptance}
    JOB=${JOB:-unit-tests}

    fly -t "$FLY_TARGET" status || fly -t "$FLY_TARGET" login

    for build in $(fly -t "$FLY_TARGET" builds -j "$PIPELINE/$JOB" | awk '/succeeded/ {print $3}'); do
        if fly -t "$FLY_TARGET" watch -j "$PIPELINE/$JOB" -b "$build" | grep '\[Fail\]' | grep "$1" >/dev/null; then
            echo "$FLY_URL/teams/$FLY_TEAM/pipelines/$PIPELINE/jobs/$JOB/builds/$build"
        fi
    done
}

main $*