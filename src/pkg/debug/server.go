package debug

import (
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
func StartServer(addr string, gatherer prometheus.Gatherer, log *logger.Logger) net.Listener {
	router := http.NewServeMux()

	router.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	router.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	router.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	router.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	router.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	router.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	server := http.Server{
		Addr:         addr,
		ReadTimeout:  2 * time.Minute,
		WriteTimeout: 2 * time.Minute,
		Handler:      router,
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error(
			"unable to setup debug server",
			err,
			logger.String("addr", addr),
		)
	}

	go func() {
		log.Info("debug server listening", logger.String("addr", lis.Addr().String()))
		log.Error("debug server closing", server.Serve(lis))
		server.Serve(lis)
	}()

	return lis
}
