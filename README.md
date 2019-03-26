Metric Store
=========
[![GoDoc][go-doc-badge]][go-doc] [![slack.cloudfoundry.org][slack-badge]][log-cache-slack]


Metric Store persists metrics from the [Loggregator System][loggregator] on disk. It is multi-tenant aware (the auth proxy ensures that you only have access to metrics from your apps), easy to query (it is 100% compatible with the Prometheus API) and has an efficient storage engine (the influx engine will make sure that your data is compressed and secured).

## Usage

Metric Store is most easily used as a BOSH release. See the [Metric Store Release](https://github.com/cloud-foundry/metric-store-release) repository for more guidance.

## Indexing

Metric Store indexes everything by the metric name on the [Loggregator Envelope][loggregator_v2]. This key allows for deterministic lookups at query-time while preventing hotspots within the cluster.

### Metric Name Compatibility

Since PromQL is used as the primary external query language for Metric Store, all metric names are rewritten to be compatible with [PromQL's naming conventions](https://prometheus.io/docs/instrumenting/writing_exporters/#metrics). If an invalid character is received, it will be converted into an underscore. No references to the old name will be retained.

### Cloud Foundry

In Cloud Foundry terms, the source ID can either represent an application
guid (e.g. `cf app <app-name> --guid`), or a component name (e.g. `doppler`).

Each request must have the `Authorization` header set with a UAA provided token.
If the token contains the `doppler.firehose` scope, the request will be able
to read data from any source ID.
If the source ID is an app guid, the Cloud Controller is consulted to verify
if the provided token has the appropriate app access.

## Query API via Gateway

Metric Store implements an interface for getting data.

### **GET** `/api/v1/query`

Issues a PromQL query against Metric Store data.

```
curl -G "http://<metric-store-addr>:8080/api/v1/query" --data-urlencode 'query=metrics{source_id="source-id-1"}'
```

##### Response Body
```
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [{ "metric": {...}, "point": [...] }]
  }
}
```
#### Notes on PromQL
A valid PromQL metric name consists of the characters [a-Z][0-9], underscore, and colon. Names can begin with [a-Z], underscore, or colon. Names cannot begin with a number [0-9].
As a measure to work with existing metrics that do not comply with the above format a conversion process takes place when matching on metric names.
As noted above, any character that is not in the set of valid characters is converted to an underscore before it is written to disk. For example, to match on a metric name `http.latency` use the name `http_latency` in your query.

[slack-badge]:              https://slack.cloudfoundry.org/badge.svg
[log-cache-slack]:          https://cloudfoundry.slack.com/archives/log-cache
[metric-store]:             https://github.com/pivotal/metric-store
[go-doc-badge]:             https://godoc.org/github.com/pivotal/metric-store?status.svg
[go-doc]:                   https://godoc.org/github.com/pivotal/metric-store
[loggregator]:              https://github.com/cloudfoundry/loggregator
[loggregator_v2]:           https://github.com/cloudfoundry/loggregator-api/blob/master/v2/envelope.proto
