package nozzle

import (
	"context"
	"crypto/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	metricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/keepalive"
	"net"
	"time"
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

	ctx, cancel := context.WithCancel(context.Background())
	// Initialize the gRPC server and register the metric service
	grpcServer := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 1 * time.Minute,
			Time:              15 * time.Second,
			Timeout:           10 * time.Second,
		}))

	// Return a new OtelServer instance containing the gRPC server and other relevant info
	return &OtelServer{
		grpcServer: grpcServer,
		log:        log,
		ms:         ms,
		ts:         ts,

		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}, 1),
	}
}

// StartGRPCServer starts the gRPC server and listens for incoming connections.
func (s *OtelServer) startGRPCServer(addr string, otelTlsConfig *tls.Config) {
	defer func() {
		close(s.done)
	}()
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		s.log.Panic("Failed to resolve address", zap.Error(err))
	}
	var listener net.Listener
	listener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		s.log.Panic("Failed to start listener", zap.Error(err))
	}

	s.addr = listener.Addr().String()

	defer listener.Close()

	for {
		listener = tls.NewListener(listener, otelTlsConfig)
		if err != nil {
			s.log.Panic("Failed to start listener for otel", zap.Error(err))
		}
		s.log.Info("Starting to listen on tcp:", logger.String("addr", addr))
		if err := s.grpcServer.Serve(listener); err != nil {
			s.log.Panic("Failed to serve gRPC server", logger.Error(err))
		}
		s.log.Info("Finished to GRPC", logger.String("addr", addr))
	}
}
