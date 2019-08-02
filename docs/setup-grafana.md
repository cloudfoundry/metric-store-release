# Set up Metric Store with Grafana

This document describes how to set up [Grafana](https://grafana.com) using
[vito/grafana-boshrelease](https://github.com/vito/grafana-boshrelease) using
UAA for authentication.

The following steps assume you are using
[cf-deployment](https://github.com/cloudfoundry/cf-deployment).

## Set up Grafana with a UAA

The following steps create a client in UAA for Grafana to use. Grafana can pass
on the user's credentials to the datasource which Grafana is querying. Grafana
therefore should be configured to use UAA as an OAuth provider.

See the following BOSH manifest operations:

```
- type: replace
  path: /variables/-
  value:
    name: grafana_client_secret
    type: password

- type: replace
  path: /instance_groups/name=uaa/jobs/name=uaa/properties/uaa/clients/grafana?
  value:
    authorities: ''
    scope: openid,uaa.resource,doppler.firehose,logs.admin,cloud_controller.read
    authorized-grant-types: authorization_code,refresh_token
    override: true
    secret: ((grafana_client_secret))
    redirect-uri: https://grafana.((system_domain))/login/generic_oauth
```

## Configure Grafana

Metric Store provides a Prometheus compatible API, therefore it can be queried
by Grafana using a Prometheus datasource.

See the following BOSH manifest operation:

```
- type: replace
  path: /instance_groups/name=metric-store/jobs/-
  value:
    name: grafana
    release: grafana
    properties:
      grafana:
        root_url: https://grafana.((system_domain))
        users:
          auto_assign_organization_role: Editor
        auth:
          generic_oauth:
            name: UAA
            enabled: true
            allow_sign_up: true
            client_id: grafana
            client_secret: ((grafana_client_secret))

            scopes:
              - openid
              - uaa.resource
              - doppler.firehose
              - logs.admin
              - cloud_controller.read

            auth_url: https://login.((system_domain))/oauth/authorize
            token_url: https://login.((system_domain))/oauth/token
            api_url: https://login.((system_domain))/userinfo
        datasources:
          - name: Metric Store
            url: https://metric-store.((system_domain))
            type: prometheus
            editable: false
            orgId: 1
            jsonData: '{"httpMethod":"GET","keepCookies":[],"oauthPassThru":true}'
```

In the above operation there are a few things to note:

- Either `doppler.firehose` or `logs.admin` scopes must be present, depending
on CF/UAA version of UAA

- `allow_sign_up` MUST be set to true, otherwise you must create accounts in
Grafana for each user in UAA

- `oauthPassThru` MUST be set, in order for Grafana to pass the auth token to
Metric Store

## Known issues

Grafana autocomplete requires `doppler.firehose` or `logs.admin` scopes, see
[#39](https://github.com/cloudfoundry/metric-store-release/issues/39)
