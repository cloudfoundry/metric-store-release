package auth

import (
	"net/http"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

func NewAccessMiddleware(accessLogger AccessLogger, host, port string, log *logger.Logger) func(http.Handler) *AccessHandler {
	return func(handler http.Handler) *AccessHandler {
		return NewAccessHandler(handler, accessLogger, host, port, log)
	}
}

func NewNullAccessMiddleware() func(http.Handler) *AccessHandler {
	return func(handler http.Handler) *AccessHandler {
		return &AccessHandler{
			handler:      handler,
			accessLogger: NewNullAccessLogger(),
		}
	}
}

type AccessHandler struct {
	handler      http.Handler
	accessLogger AccessLogger
	host         string
	port         string
	log          *logger.Logger
}

func NewAccessHandler(handler http.Handler, accessLogger AccessLogger, host, port string, log *logger.Logger) *AccessHandler {
	return &AccessHandler{
		handler:      handler,
		accessLogger: accessLogger,
		host:         host,
		port:         port,
		log:          log,
	}
}

func (h *AccessHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if err := h.accessLogger.LogAccess(req, h.host, h.port); err != nil {
		h.log.Error("access handler error", err)
	}

	h.handler.ServeHTTP(rw, req)
}
