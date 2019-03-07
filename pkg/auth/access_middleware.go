package auth

import (
	"log"
	"net/http"
)

func NewAccessMiddleware(accessLogger AccessLogger, host, port string) func(http.Handler) *AccessHandler {
	return func(handler http.Handler) *AccessHandler {
		return NewAccessHandler(handler, accessLogger, host, port)
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

type AccessLogger interface {
	LogAccess(req *http.Request, host, port string) error
}

type AccessHandler struct {
	handler      http.Handler
	accessLogger AccessLogger
	host         string
	port         string
}

func NewAccessHandler(handler http.Handler, accessLogger AccessLogger, host, port string) *AccessHandler {
	return &AccessHandler{
		handler:      handler,
		accessLogger: accessLogger,
		host:         host,
		port:         port,
	}
}

func (h *AccessHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if err := h.accessLogger.LogAccess(req, h.host, h.port); err != nil {
		log.Printf("access handler: %s", err)
	}

	h.handler.ServeHTTP(rw, req)
}
