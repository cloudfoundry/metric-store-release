Metric Store
=========
[![GoDoc][go-doc-badge]][go-doc] [![slack.cloudfoundry.org][slack-badge]][log-cache-slack]


Metric Store persists metrics from the [Loggregator System][loggregator] on disk. It is multi-tenant aware (the auth proxy ensures that you only have access to metrics from your apps), easy to query (it is 100% compatible with the Prometheus API) and has an efficient storage engine (the influx engine will make sure that your data is compressed and secured).

## Deploying

Metric Store can be deployed within [Cloud Foundry](https://github.com/cloudfoundry/cf-deployment). Metric Store will have to know about Loggregator.

### Cloud Config

Every BOSH deployment requires a [cloud config](https://bosh.io/docs/cloud-config.html). The Metric Store deployment manifest assumes the CF-Deployment cloud config has been uploaded.

### Creating and Uploading Release

The first step in deploying Metric Store is to create a release. Final releases are preferable, however during the development process dev releases are useful.

The following commands will create a dev release and upload it to an environment named `lite`.

```
bosh create-release --force
bosh -e lite upload-release --rebase
```

### Cloud Foundry

Metric Store deployed within Cloud Foundry reads from the Loggregator system and registers with the [GoRouter](https://github.com/cloudfoundry/gorouter) at `metric-store.<system-domain>` (e.g. for bosh-lite `metric-store.bosh-lite.com`).

The following commands will deploy Metric Store in CF.

```
bosh update-runtime-config \
    ~/workspace/bosh-deployment/runtime-configs/dns.yml
bosh update-cloud-config \
    ~/workspace/cf-deployment/iaas-support/bosh-lite/cloud-config.yml
bosh \
    --environment lite \
    --deployment cf \
    deploy ~/workspace/cf-deployment/cf-deployment.yml \
    --ops-file ~/workspace/cf-deployment/operations/bosh-lite.yml \
    --ops-file ~/workspace/cf-deployment/operations/use-compiled-releases.yml \
    --ops-file ./manifests/ops-files/add-metric-store-to-cfd.yml
    -v system_domain=bosh-lite.com
```

### Metric Store Client
By Default, Metric Store uses the `doppler` client included with `cf-deployment`.

If you would like to use a custom client, it requires the `uaa.resource` authority:
```
<custom_client_id>:
    authorities: uaa.resource
    override: true
    authorized-grant-types: client_credentials
    secret: <custom_client_secret>
```

## Operating Metric Store
Metric Store is currently an experimental release with plans for integrations startingin early 2019.

#### Reliability SLO
Metric Store depends on Loggregator and is thus is tied to its reliability ceiling. Metric Store currently implements hinted handoff as a strategy for queueing writes to remote nodes that are offline due to restarts, and thus aims to successfully write 99.9% of all envelopes received from Loggregator.

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
