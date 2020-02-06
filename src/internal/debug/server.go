package debug

import (
	"crypto/tls"
	"expvar"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// StartServer listens and serves the health endpoint HTTP handler on a given
// address. If the server fails to listen or serve the process will exit with
// a status code of 1.
func StartServer(addr string, tlsConfig *tls.Config, gatherer prometheus.Gatherer, log *logger.Logger) net.Listener {
	router := http.NewServeMux()

	router.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
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
		TLSConfig:    tlsConfig,
	}

	insecureConnection, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error(
			"unable to setup debug server insecure connection",
			err,
			logger.String("addr", addr),
		)
	}

	secureConnection := tls.NewListener(insecureConnection, tlsConfig)

	go func() {
		log.Info("debug server listening", logger.String("addr", secureConnection.Addr().String()))
		server.Serve(secureConnection)
		log.Info("debug server closing")
	}()

	return secureConnection
}
