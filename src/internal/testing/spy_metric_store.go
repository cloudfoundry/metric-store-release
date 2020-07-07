package testing

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"

	"go.uber.org/atomic"

	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"

	. "github.com/onsi/gomega"
)

type SpyMetricStore struct {
	mu              sync.Mutex
	localOnlyValues []bool

	ReloadRequestsCount       atomic.Int32
	queryResultValue          float64
	QueryError                error
	ingressListener           *SpyTCPListener
	remoteInternodeConnection *SpyTCPListener

	sentPoints []*rpc.Point

	tlsConfig *tls.Config
}

type SpyMetricStoreAddrs struct {
	EgressAddr    string
	IngressAddr   string
	InternodeAddr string
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
	s.ingressListener = NewSpyTCPListener(s.tlsConfig)
	err := s.ingressListener.Start()
	if err != nil {
		fmt.Printf("failed to listen: %v", err)
	}

	s.remoteInternodeConnection = NewSpyTCPListener(s.tlsConfig)
	err = s.remoteInternodeConnection.Start()
	if err != nil {
		fmt.Printf("failed to listen: %v", err)
	}

	// egress connection
	insecureConnection, err := net.Listen("tcp", ":0")
	Expect(err).ToNot(HaveOccurred())
	secureConnection := tls.NewListener(insecureConnection, s.tlsConfig)

	mux := http.NewServeMux()
	mux.Handle("/~/reload", s.handleReload())
	mux.Handle("/", s.handleDefault())
	egressServer := &http.Server{
		Handler: mux,
	}

	go egressServer.Serve(secureConnection)

	return SpyMetricStoreAddrs{
		EgressAddr:    secureConnection.Addr().String(),
		IngressAddr:   s.ingressListener.Address(),
		InternodeAddr: s.remoteInternodeConnection.Address(),
	}
}

func (s *SpyMetricStore) handleReload() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.ReloadRequestsCount.Inc()
	})
}

func (s *SpyMetricStore) handleDefault() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("got uncaught path: " + r.RequestURI)
	})
}

func (s *SpyMetricStore) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ingressListener.Stop()
	s.remoteInternodeConnection.Stop()
}

func (s *SpyMetricStore) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.ingressListener.Resume()
	if err != nil {
		fmt.Printf("failed to restart ingress listener: %v", err)
	}

	err = s.remoteInternodeConnection.Resume()
	if err != nil {
		fmt.Printf("failed to restart internode listener: %v", err)
	}
}

func (s *SpyMetricStore) GetPoints() []*rpc.Point {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ingressListener.GetPoints()
}

func (s *SpyMetricStore) GetInternodePoints() []*rpc.Point {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.remoteInternodeConnection.GetPoints()
}

func (s *SpyMetricStore) GetLocalOnlyValues() []bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := make([]bool, len(s.localOnlyValues))
	copy(r, s.localOnlyValues)
	return r
}

func (s *SpyMetricStore) Addr() string {
	return s.remoteInternodeConnection.Address()
}
