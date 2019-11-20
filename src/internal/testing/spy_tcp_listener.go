package testing

import (
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/niubaoshu/gotiny"
)

type SpyTCPListener struct {
	mu                 sync.Mutex
	listener           *leanstreams.TCPListener
	receivedPointsChan chan *rpc.Point
	receivedPoints     []*rpc.Point
	tlsConfig          *tls.Config
}

func NewSpyTCPListener(tlsConfig *tls.Config) *SpyTCPListener {
	return &SpyTCPListener{
		tlsConfig:          tlsConfig,
		receivedPointsChan: make(chan *rpc.Point, 1000),
	}
}

func (s *SpyTCPListener) Start() error {
	callback := func(payload []byte) error {
		batch := rpc.Batch{}
		gotiny.Unmarshal(payload, &batch)

		for _, e := range batch.Points {
			s.receivedPointsChan <- e
		}

		return nil
	}

	cfg := leanstreams.TCPListenerConfig{
		MaxMessageSize: ingressclient.MAX_INGRESS_PAYLOAD_SIZE_IN_BYTES,
		Callback:       callback,
		Address:        ":0",
		TLSConfig:      s.tlsConfig,
	}
	listener, err := leanstreams.ListenTCP(cfg)
	s.listener = listener
	if err != nil {
		fmt.Printf("failed to listen: %v", err)
		return err
	}

	err = listener.StartListeningAsync()
	if err != nil {
		fmt.Printf("failed to start async listening: %v", err)
		return err
	}

	return nil
}

func (s *SpyTCPListener) Address() string {
	return s.listener.Address
}

func (s *SpyTCPListener) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listener.Close()
}

func (s *SpyTCPListener) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.listener.RestartListeningAsync()
	if err != nil {
		fmt.Printf("failed to restart listener: %v", err)
		return err
	}
	return nil
}

func (s *SpyTCPListener) GetPoints() []*rpc.Point {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		select {
		case point := <-s.receivedPointsChan:
			s.receivedPoints = append(s.receivedPoints, point)
		default:
			return s.receivedPoints
		}
	}
}
