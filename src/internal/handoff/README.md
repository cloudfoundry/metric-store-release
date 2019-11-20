# Hinted Handoff Queue

Hinted handoff is a concept that is used in
[Cassandra](https://docs.datastax.com/en/cassandra/3.0/cassandra/operations/opsRepairNodesHintedHandoff.html)
and [Amazon's Dynamo](https://www.allthingsdistributed.com/2007/10/amazons_dynamo.html)
for dealing with failed remote writes.

The files `queue.go`, `limiter.go` and `node_processor.go` (and their tests) were taken
from InfluxDB 0.11.1:

https://github.com/influxdata/influxdb/tree/v0.11.1/services/hh

These files were released under the MIT license.
