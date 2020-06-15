package metrics

import (
	"crypto/tls"
	"expvar"
	"net"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

type Server struct {
	listener  net.Listener
	registrar Registrar

	mu sync.Mutex // TODO needed?
}

func (s *Server) Close() error {
	if s.listener == nil {
		return nil
	}
	return s.listener.Close()
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) Registrar() Registrar {
	return s.registrar
}

// StartMetricsServer listens and serves the health endpoint HTTP handler on a given
// address. If the server fails to listen or serve the process will exit with
// a status code of 1.
func StartMetricsServer(addr string, tlsConfig *tls.Config, log *logger.Logger, registrar Registrar) *Server {
	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.HandlerFor(registrar.Gatherer(), promhttp.HandlerOpts{}))

	server := http.Server{
		Addr:         addr,
		ReadTimeout:  2 * time.Minute,
		WriteTimeout: 2 * time.Minute,
		Handler:      router,
		TLSConfig:    tlsConfig,
	}

	insecureConnection, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error(
			"unable to setup metrics server insecure connection",
			err,
			logger.String("addr", addr),
		)
		// TODO control flow
	}

	secureConnection := tls.NewListener(insecureConnection, tlsConfig)

	go func() {
		log.Info("metrics server listening", logger.String("addr", secureConnection.Addr().String()))
		server.Serve(secureConnection) // TODO err
		log.Info("metrics server closing")
	}()

	return &Server{
		listener:     secureConnection,
		registrar:    registrar,
	}
}

func StartProfilingServer(addr string, log *logger.Logger) net.Listener {
	router := http.NewServeMux()

	router.HandleFunc("/debug/pprof/", pprof.Index)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)
	router.Handle("/debug/vars", expvar.Handler())

	server := http.Server{
		Addr:         addr,
		ReadTimeout:  2 * time.Minute,
		WriteTimeout: 2 * time.Minute,
		Handler:      router,
	}

	insecureConnection, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error(
			"unable to setup debug server insecure connection",
			err,
			logger.String("addr", addr),
		)
		return nil
	}

	go func() {
		log.Info("debug server listening", logger.String("addr", insecureConnection.Addr().String()))
		server.Serve(insecureConnection) // TODO err, but let the linter tell us first
		log.Info("debug server closing")
	}()

	return insecureConnection
}
