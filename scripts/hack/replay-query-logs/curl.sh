#!/usr/bin/env bash

export BOSH_DEPLOYMENT=metric-store-edge
export TOKEN="$(cf oauth-token)"

# TODO use bosh logs instead
for i in $(bosh vms --column Instance | awk -F / '{ print $2 }'); do
    bosh scp metric-store/$i:/var/vcap/sys/log/metric-store/query.log $i.log
done

# TODO - if we need to run continuously, write the filter output to a file and refresh the token between runs
sort -u *.log | jq -r -f ./filter.jq | parallel -j 25 "curl -H \"Authorization: $TOKEN\" -sS -G {}"