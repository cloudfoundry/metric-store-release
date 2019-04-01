# Metric Store
[![slack.cloudfoundry.org][slack-badge]][slack-channel]

Metric Store Release is a [BOSH][bosh] release for Metric Store. It provides a
persistent storage layer for metrics sent through the Loggregator subsystem.

## Deploying Metric Store

Metric Store can be deployed within [Cloud Foundry][cfd] by using this
[operation file][ops-file].

```
bosh -e my-bosh -d CF \
    deploy [cf-deployment.yml][cfd-manifest] \
    -o [add-metric-store-to-cfd.yml][ops-file]
```

Metric Store registers with the [GoRouter][go-router] at
`metric-store.<system-domain>`.

#### Metric Store Client
By Default, Metric Store uses the `doppler` client included with `cf-deployment`.

If you would like to use a custom client, it requires the `uaa.resource` authority:
```
<custom_client_id>:
    authorities: uaa.resource
    override: true
    authorized-grant-types: client_credentials
    secret: <custom_client_secret>
```

#### Creating and Uploading Release

The first step in deploying Metric Store is to create a release or download from [bosh.io][bosh-io-release].

The following commands will create a dev release and upload it to an
environment named `my-bosh`.

```
bosh create-release --force
bosh -e my-bosh upload-release --rebase
```

## Using Metric Store
### Storing Metrics
Metric Store ingresses all metrics (eliminating logs) from the Reverse Log
Proxy on Loggregator. Any metric sent to a Loggregator Agent will travel
downstream into Metric Store.

### Accessing Metrics in Metric Store
#### Authorization and Authentication
Metric Store as deployed in a Cloud Foundry deployment depends on the
`CF Auth Proxy` job to convert your UAA provided auth token into an authorized
list of source IDs for Metric Store. In order to see all metrics, the token
used must have the `logs.admin` scope.

#### PromQL via HTTP Client
Metric Store provides Prometheus Query Language (PromQL) compatible endpoints.
Queries against Metric Store can be crafted using [Prometheus API
Documentation][promql].

##### Example
Issues a PromQL query against Metric Store data.
```
curl -G "http://metric-store.<system-domain>/api/v1/query" --data-urlencode 'query=metrics{source_id="source-id-1"}'
```
See [PromQL documentation][promql] for more.

##### Notes on PromQL
A valid PromQL metric name consists of the characters [a-Z][0-9], underscore, and colon. Names can begin with [a-Z], underscore, or colon. Names cannot begin with a number [0-9].
As a measure to work with existing metrics that do not comply with the above format a conversion process takes place when matching on metric names.
As noted above, any character that is not in the set of valid characters is converted to an underscore before it is written to disk. For example, to match on a metric name `http.latency` use the name `http_latency` in your query.

#### gRPC Client For Bosh Deployed Components
Interacting with Metric Store directly, circumventing the GoRouter and CF Auth
Proxy, can be done using our [Go client library][client] or by generating your
own with the `.proto` files. This will require a bosh deployed component to
receive `metric-store` bosh links for certificate sharing. The resulting
client interaction has admin access.

### Reliability SLO
Metric Store depends on Loggregator and is thus is tied to its reliability ceiling. Metric Store
currently implements hinted handoff as a strategy for queueing writes to remote nodes that are offline
due to restarts, and thus aims to successfully write 99.9% of all envelopes received from Loggregator.

[slack-badge]:     https://slack.cloudfoundry.org/badge.svg
[slack-channel]:   https://cloudfoundry.slack.com/archives/metric-store
[bosh]:            https://github.com/cloudfoundry/bosh
[cfd]:             https://github.com/cloudfoundry/cf-deployment
[cfd-manifest]:    https://github.com/cloudfoundry/cf-deployment/blob/master/cf-deployment.yml
[ops-file]:        https://github.com/cloudfoundry/metric-store-release/blob/master/manifests/ops-files/add-metric-store-to-cfd.yml
[go-router]:       https://github.com/cloudfoundry/gorouter
[bosh-io-release]: https://bosh.io/releases/github.com/cloudfoundry/metric-store-release?latest
[promql]:          https://prometheus.io/docs/prometheus/latest/querying/api/
