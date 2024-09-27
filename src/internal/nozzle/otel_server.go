package nozzle

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	metricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"go.uber.org/zap"

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

	// Reset s.grpcServer to nil to indicate it's closed
	s.grpcServer = nil

	s.cancel()
	<-s.done

	return nil
}

func (s *OtelServer) Start(addr string, otelTlsConfig *tls.Config) {
	s.log.Info("OtelServer starting grpc server at ", zap.String("address", addr))

	// Initialize gRPC server with TLS credentials
	grpcServer := grpc.NewServer()
	// Start the gRPC server in a separate goroutine
	go s.startGRPCServer(addr, grpcServer, otelTlsConfig)

	// Register Metric and Trace services with the gRPC server
	metricspb.RegisterMetricsServiceServer(grpcServer, s.ms)
	tracepb.RegisterTraceServiceServer(grpcServer, s.ts)

	s.log.Info("Starting Metrics and Trace Servers")
	s.ms.StartListening()
	s.ts.StartListening()
	s.log.Info("OtelServer started")
}

func NewOtelServer(
	log *logger.Logger,
	ms *MetricService,
	ts *TraceService,
) *OtelServer {
	ctx, cancel := context.WithCancel(context.Background())

	// Return a new OtelServer instance containing the gRPC server and other relevant info
	return &OtelServer{
		log:    log,
		ms:     ms,
		ts:     ts,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}, 1),
	}
}

// StartGRPCServer starts the gRPC server and listens for incoming connections.
func (s *OtelServer) startGRPCServer(addr string, grpcServer *grpc.Server, otelTlsConfig *tls.Config) {
	defer func() {
		close(s.done)
	}()

	address := fmt.Sprintf("0.0.0.0:%s", addr)
	resolvedAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		s.log.Panic("Failed to resolve address", zap.Error(err))
	}
	listener, err := tls.Listen("tcp", resolvedAddr.String(), otelTlsConfig)
	if err != nil {
		s.log.Panic("Failed to start listener", zap.Error(err))
	}
	defer listener.Close()

	s.log.Info("Listening on", zap.String("address", resolvedAddr.String()))

	// Serve the gRPC server on the TCP listener
	if err := grpcServer.Serve(listener); err != nil {
		s.log.Panic("gRPC server failed", zap.Error(err))
	}
}
