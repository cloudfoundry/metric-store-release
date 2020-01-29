package ingressclient

import (
	"bytes"
	"crypto/tls"
	"encoding/gob"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
)

const (
	MAX_BATCH_SIZE_IN_BYTES           = 32 * 1024
	MAX_INGRESS_PAYLOAD_SIZE_IN_BYTES = 2 * MAX_BATCH_SIZE_IN_BYTES
)

type IngressClient struct {
	connection  *leanstreams.TCPClient
	log         *logger.Logger
	dialTimeout time.Duration
}

func NewIngressClient(ingressAddress string, tlsConfig *tls.Config, opts ...IngressClientOption) (*IngressClient, error) {
	client := &IngressClient{
		log:         logger.NewNop(),
		dialTimeout: 10 * time.Second,
	}

	for _, o := range opts {
		o(client)
	}

	clientConfig := &leanstreams.TCPClientConfig{
		MaxMessageSize: MAX_INGRESS_PAYLOAD_SIZE_IN_BYTES,
		Address:        ingressAddress,
		TLSConfig:      tlsConfig,
	}

	connection, err := leanstreams.DialTCPUntilConnected(clientConfig, client.dialTimeout)
	if err != nil {
		return nil, err
	}
	client.connection = connection

	return client, nil
}

type IngressClientOption func(*IngressClient)

func WithIngressClientLogger(log *logger.Logger) IngressClientOption {
	return func(client *IngressClient) {
		client.log = log
	}
}

func WithDialTimeout(timeout time.Duration) IngressClientOption {
	return func(client *IngressClient) {
		client.dialTimeout = timeout
	}
}

func (c *IngressClient) Write(points []*rpc.Point) error {
	var payload bytes.Buffer
	enc := gob.NewEncoder(&payload)
	err := enc.Encode(rpc.Batch{Points: points})
	if err != nil {
		c.log.Error("gob encode error", err)
		return err
	}

	// TODO: consider adding back in a timeout (i.e. 3 seconds)
	bytesWritten, err := c.connection.Write(payload.Bytes())

	if err == nil {
		c.log.Info("wrote bytes", logger.Count(bytesWritten))
	}

	return err
}

func (c *IngressClient) Close() {
	c.connection.Close()
}
