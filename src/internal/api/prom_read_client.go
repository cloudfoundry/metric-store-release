package api

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/storage/remote"
)

func NewPromReadClient(index int, addr string, timeout time.Duration,
	tlsConfig *config.TLSConfig) (remote.ReadClient, error) {
	clientAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}

	url := &url.URL{
		Scheme: "https",
		Host:   clientAddr.String(),
		Path:   "/api/v1/read",
	}

	c, err := remote.NewReadClient(
		fmt.Sprintf("Node %d", index),
		&remote.ClientConfig{
			URL:     &config.URL{URL: url},
			Timeout: model.Duration(timeout),
			HTTPClientConfig: config.HTTPClientConfig{
				TLSConfig: *tlsConfig,
			},
		},
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}
