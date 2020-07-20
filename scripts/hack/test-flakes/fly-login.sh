#!/usr/bin/env bash

if ! fly targets|grep UTC >/dev/null; then
    echo "Please log in to a Concourse instance"
    exit 127
fi

export FLY_TARGET="$(fly targets|awk '/UTC/ {print $1}'|head -1)"
export FLY_URL="$(fly targets|awk '/UTC/ {print $2}'|head -1)"
export FLY_TEAM="$(fly targets|awk '/UTC/ {print $3}'|head -1)"