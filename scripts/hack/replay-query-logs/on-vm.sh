#!/usr/bin/env bash

cd /tmp
apt-get -y install jq parallel

echo <<EOF >filter.jq
. |
select( .ts > "2020-07-01T16:30") | # TODO take timestamp as param?
select( .params.step != 0) | # step=0 is query instead of query_range (I think?)
@uri "https://localhost:8080/private/api/v1/query_range?query=\(.params.query)&start=\(.params.start)&end=\(.params.end)&step=\(.params.step)"
EOF

sort -u /var/vcap/sys/log/metric-store/query.log | jq -r -f ./filter.jq >queries

cat ./queries | parallel -j25 "curl -sS -G {} \
    --cacert /var/vcap/jobs/metric-store/config/certs/metric_store_ca.crt \
    --key /var/vcap/jobs/metric-store/config/certs/metric_store.key \
    --cert /var/vcap/jobs/metric-store/config/certs/metric_store.crt "