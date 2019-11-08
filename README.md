# Metric Store: A Cloud-Native Time Series Database
[![slack.cloudfoundry.org][slack-badge]][slack-channel] [![Build Status](https://travis-ci.org/cloudfoundry/metric-store-release.svg?branch=develop)](https://travis-ci.org/cloudfoundry/metric-store-release)

Metric Store Release is a [BOSH][bosh] release for Metric Store. It provides a persistent storage layer for metrics sent through the Loggregator subsystem. It is multi-tenant aware (the auth proxy ensures that you only have access to metrics from your apps), easy to query (it is 100% compatible with the Prometheus Query API, with some exceptions listed below), and has a powerful storage engine (the InfluxDB storage engine has built-in compression and a memory-efficient series index).

## Deploying

Metric Store can be deployed within [Cloud Foundry][cfd]. Metric Store will have to know about Loggregator.

### Cloud Config

Every BOSH deployment requires a [cloud config](https://bosh.io/docs/cloud-config.html). The Metric Store deployment manifest assumes the CF-Deployment cloud config has been uploaded.

### Creating and Uploading a Release

The first step in deploying Metric Store is to create a release or download it from [bosh.io][bosh-io-release]. Final releases are preferable, however during the development process dev releases are useful.

The following commands will create a dev release and upload it to an environment named `testing`.
```
bosh create-release --force
bosh -e testing upload-release --rebase
```

### Cloud Foundry

Metric Store deployed within Cloud Foundry reads from the Loggregator system and registers with the [GoRouter](https://github.com/cloudfoundry/gorouter) at `metric-store.<system-domain>`.

You can deploy Metric Store by using this
[operations file][ops-file].

```
bosh -e testing -d cf \
    deploy cf-deployment.yml \
    -o add-metric-store-to-cfd.yml
```

### Metric Store UAA Client
By Default, Metric Store uses the `doppler` client included with `cf-deployment`.

If you would like to use a custom client, it requires the `uaa.resource` authority:
```
<custom_client_id>:
    authorities: uaa.resource
    override: true
    authorized-grant-types: client_credentials
    secret: <custom_client_secret>
```

## Using Metric Store

### Storing Metrics
Metric Store ingresses all metrics (discarding logs) from the Reverse Log
Proxy on Loggregator. Any metric sent to a Loggregator Agent will travel
downstream into Metric Store.

### Accessing Metrics in Metric Store

#### Authorization and Authentication
Metric Store as deployed in a Cloud Foundry deployment depends on the
`CF Auth Proxy` job to convert your UAA provided auth token into an authorized
list of source IDs for Metric Store. In Cloud Foundry terms, the source ID can either represent an application
guid (e.g. `cf app <app-name> --guid`), or a component name (e.g. `doppler`).

Each request must have the `Authorization` header set with a UAA provided token.
If the token contains the `doppler.firehose` scope, the request will be able
to read data from any source ID.
If the source ID is an app guid, the Cloud Controller is consulted to verify
if the provided token has the appropriate app access.

#### PromQL via HTTP
Metric Store provides Prometheus Query Language (PromQL) compatible endpoints.
Queries against Metric Store can be crafted with the help of the [Prometheus API
Documentation][promql].

##### Example: **GET** `/api/v1/query`

This issues a PromQL query against Metric Store data.

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
See the official [PromQL API documentation][promql] for more information.

##### Notes on PromQL
A valid PromQL metric name consists of the characters [a-Z][0-9], underscore, and colon. Names can begin with [a-Z], underscore, or colon. Names cannot begin with a number [0-9].
As a measure to work with existing metrics that do not comply with the above format a conversion process takes place when matching on metric names.
As noted above, any character that is not in the set of valid characters is converted to an underscore before it is written to disk. For example, to match on a metric name `http.latency` use the name `http_latency` in your query.

##### Prometheus API Compatability
- `/api/v1/query` & `/api/v1/query_range`, fully supported except for regex
  matchers on `__name__` (for everyone) or `source_id` (for non-admins)
- `/api/v1/series`, `/api/v1/labels`, `/api/v1/rules`, `/api/v1/alerts` &
  /api/v1/alertmanagers, fully supported for admins
- the remaining endpoints are not currently supported

#### Golang Clients For BOSH-Deployed Components
Interacting with Metric Store directly, circumventing the GoRouter and CF Auth
Proxy, can be done using our [Go ingress client library][ingressclient] or our
[Go egress client library][egressclient]. This will require a bosh deployed
component to receive `metric-store` bosh links for certificate sharing. The
resulting client interaction has admin access.

#### Using Grafana to visualize metrics

See [Set up Metric Store with Grafana](/docs/setup-grafana.md) in the docs
directory.

## Contributing

We'd love to hear feedback about your experiences with Metric Store. Please feel free to open up an [issue][issues], send us a [pull request][prs], or come chat with us on [Cloud Foundry Slack][slack-channel].

[slack-badge]:     https://slack.cloudfoundry.org/badge.svg
[slack-channel]:   https://cloudfoundry.slack.com/archives/metric-store
[bosh]:            https://github.com/cloudfoundry/bosh
[cfd]:             https://github.com/cloudfoundry/cf-deployment
[cfd-manifest]:    https://github.com/cloudfoundry/cf-deployment/blob/master/cf-deployment.yml
[ops-file]:        https://github.com/cloudfoundry/metric-store-release/blob/master/manifests/ops-files/add-metric-store-to-cfd.yml
[go-router]:       https://github.com/cloudfoundry/gorouter
[bosh-io-release]: https://bosh.io/releases/github.com/cloudfoundry/metric-store-release?latest
[promql]:          https://prometheus.io/docs/prometheus/latest/querying/api/
[ingressclient]:   https://github.com/cloudfoundry/metric-store-release/tree/develop/src/pkg/ingressclient
[egressclient]:    https://github.com/cloudfoundry/metric-store-release/tree/develop/src/pkg/egressclient
[issues]:          https://github.com/cloudfoundry/metric-store-release/issues
[prs]:             https://github.com/cloudfoundry/metric-store-release/pulls
