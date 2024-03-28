package nozzle

import (
	"crypto/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	metricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"golang.org/x/net/context"
	_ "google.golang.org/grpc/encoding/gzip"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"google.golang.org/grpc"
)

type OtelServer struct {
	grpcServer *grpc.Server
	otelTls    *tls.Config
	log        *logger.Logger
	addr       string
	ms         *MetricService
	ts         *TraceService

	ctx    context.Context
	cancel func()
	done   chan struct{}
}

func (s *OtelServer) Close() error {
	if s.grpcServer == nil {
		// Nothing to close if the server was never initialized or already closed
		return nil
	}

	// Gracefully stop the gRPC server
	s.grpcServer.GracefulStop()

	s.ms.Stop()
	s.ts.Stop()

	// Optionally, reset s.grpcServer to nil to indicate it's closed
	// This is useful if you're checking the state of the server elsewhere
	s.grpcServer = nil

	s.cancel()
	<-s.done

	return nil
}

func (s *OtelServer) Start(addr string, otelTlsConfig *tls.Config) {
	s.log.Info("OtelServer starting grpc server")
	go s.startGRPCServer(addr, otelTlsConfig)

	s.log.Info("Registering Metrics and Trace Server")
	metricspb.RegisterMetricsServiceServer(s.grpcServer, s.ms)
	tracepb.RegisterTraceServiceServer(s.grpcServer, s.ts)

	s.log.Info("Starting Metrics Server")
	s.ms.StartListening()
	s.log.Info("Starting Trace Server")
	s.ts.StartListening()
	s.log.Info("OtelServer started")

}

func NewOtelServer(
	log *logger.Logger,
	ms *MetricService,
	ts *TraceService,
) *OtelServer {

	// Initialize the gRPC server and register the metric service
	grpcServer := grpc.NewServer()

	// Return a new OtelServer instance containing the gRPC server and other relevant info
	return &OtelServer{
		grpcServer: grpcServer,
		log:        log,
		ms:         ms,
		ts:         ts,
	}
}

// StartGRPCServer starts the gRPC server and listens for incoming connections.
func (s *OtelServer) startGRPCServer(addr string, otelTlsConfig *tls.Config) {
	defer func() {
		close(s.done)
	}()

	noop := func(payload []byte) error {
		println("Not expecting this to be called: %v", payload)
		return nil
	}

	tcpConfig := leanstreams.TCPListenerConfig{
		Address:   addr,
		Callback:  noop,
		TLSConfig: otelTlsConfig,
	}
	lis, err := leanstreams.ListenTCP(tcpConfig)
	if err != nil {
		s.log.Panic("Failed to configure tcp", logger.String("addr", addr), logger.Error(err))
	}

	s.log.Info("otel server listening", logger.String("addr", lis.Addr().String()))
	if err := s.grpcServer.Serve(lis); err != nil {
		s.log.Panic("Failed to serve gRPC server", logger.Error(err))
	}
	err = lis.StartListeningAsync()
	if err != nil {
		s.log.Panic("Failed to start listening on tcp", logger.String("addr", addr), logger.Error(err))
	}
}
