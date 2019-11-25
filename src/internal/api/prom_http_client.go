package api

import (
	"crypto/tls"
	"net/http"
	"net/url"

	prom_http_client "github.com/prometheus/client_golang/api"
	prom_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
)

func NewPromHTTPClient(addr, path string, tlsConfig *tls.Config) (prom_api_client.API, error) {
	url := &url.URL{Scheme: "https", Host: addr, Path: path}

	client, err := prom_http_client.NewClient(
		prom_http_client.Config{
			Address:      url.String(),
			RoundTripper: &http.Transport{TLSClientConfig: tlsConfig},
		},
	)
	if err != nil {
		return nil, err
	}

	return prom_api_client.NewAPI(client), nil
}
