package testing

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/gomega"
)

type SpyMetricStore struct {
	mu sync.Mutex

	queryResultValue float64
	QueryError       error
	remoteConnection *leanstreams.TCPListener

	sentPoints []*rpc.Point

	tlsConfig *tls.Config
}

type SpyMetricStoreAddrs struct {
	EgressAddr  string
	IngressAddr string
}

func (s *SpyMetricStore) SetValue(value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.queryResultValue = value
}

func NewSpyMetricStore(tlsConfig *tls.Config) *SpyMetricStore {
	return &SpyMetricStore{
		tlsConfig:        tlsConfig,
		queryResultValue: 101,
	}
}

func (s *SpyMetricStore) Start() SpyMetricStoreAddrs {
	// TCP/ingress connection
	callback := func(payload []byte) error {
		s.mu.Lock()
		defer s.mu.Unlock()

		r := &rpc.SendRequest{}

		err := proto.Unmarshal(payload, r)
		if err != nil {
			return err
		}

		for _, e := range r.Batch.Points {
			s.sentPoints = append(s.sentPoints, e)
		}

		return nil
	}

	cfg := leanstreams.TCPListenerConfig{
		MaxMessageSize: ingressclient.MAX_INGRESS_PAYLOAD_SIZE_IN_BYTES,
		Callback:       callback,
		Address:        ":0",
		TLSConfig:      s.tlsConfig,
	}
	btl, err := leanstreams.ListenTCP(cfg)
	s.remoteConnection = btl
	if err != nil {
		fmt.Printf("failed to listen: %v", err)
	}

	err = btl.StartListeningAsync()
	if err != nil {
		fmt.Printf("failed to start async listening: %v", err)
	}

	// egress connection
	insecureConnection, err := net.Listen("tcp", ":0")
	Expect(err).ToNot(HaveOccurred())
	secureConnection := tls.NewListener(insecureConnection, s.tlsConfig)
	egressServer := &http.Server{}
	go egressServer.Serve(secureConnection)

	return SpyMetricStoreAddrs{
		EgressAddr:  secureConnection.Addr().String(),
		IngressAddr: btl.Address,
	}
}

func (s *SpyMetricStore) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.remoteConnection.Close()
}

func (s *SpyMetricStore) GetPoints() []*rpc.Point {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]*rpc.Point, len(s.sentPoints))
	copy(r, s.sentPoints)
	return r
}

func (s *SpyMetricStore) Send(ctx context.Context, r *rpc.SendRequest) (*rpc.SendResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range r.Batch.Points {
		s.sentPoints = append(s.sentPoints, e)
	}

	return &rpc.SendResponse{}, nil
}
