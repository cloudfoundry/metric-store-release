package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/debug"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/golang/protobuf/jsonpb"
	"github.com/gorilla/mux"
)

type CFAuthMiddlewareProvider struct {
	log     *logger.Logger
	metrics MetricRegistrar

	oauth2Reader  Oauth2ClientReader
	logAuthorizer LogAuthorizer
	queryParser   QueryParser
	marshaller    jsonpb.Marshaler
}

type Oauth2ClientContext struct {
	IsAdmin   bool
	Token     string
	ExpiresAt time.Time
}

func NewCFAuthMiddlewareProvider(
	oauth2Reader Oauth2ClientReader,
	logAuthorizer LogAuthorizer,
	queryParser QueryParser,
	metrics MetricRegistrar,
	log *logger.Logger,
) CFAuthMiddlewareProvider {
	return CFAuthMiddlewareProvider{
		oauth2Reader:  oauth2Reader,
		logAuthorizer: logAuthorizer,
		queryParser:   queryParser,
		metrics:       metrics,
		log:           log,
	}
}

type promqlErrorBody struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

func (m CFAuthMiddlewareProvider) Middleware(h http.Handler) http.Handler {
	router := mux.NewRouter()

	router.HandleFunc("/api/v1/{subpath:query|query_range}", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		authToken := r.Header.Get("Authorization")
		if authToken == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		userContext, err := m.oauth2Reader.Read(authToken)
		if err != nil {
			m.log.Error("failed to read from Oauth2 server", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if !userContext.IsAdmin {
			query := r.URL.Query().Get("query")
			relevantSourceIds, err := m.queryParser.ExtractSourceIds(query)
			if err != nil {
				http.Error(w, fmt.Sprintf("query parse error: %s", err), http.StatusUnprocessableEntity)
				return
			}
			if !m.authorized(relevantSourceIds, userContext) {
				http.Error(w, fmt.Sprintf("there are no matching source IDs for your query"), http.StatusUnauthorized)
				return
			}
		}

		h.ServeHTTP(w, r)

		totalQueryTime := time.Since(start).Seconds()
		m.metrics.Set(debug.AuthProxyRequestDurationSeconds, float64(totalQueryTime))

	})

	router.HandleFunc("/api/v1/labels", func(w http.ResponseWriter, r *http.Request) {
		m.handleOnlyAdmin(h, w, r)
	})

	router.HandleFunc("/api/v1/series", func(w http.ResponseWriter, r *http.Request) {
		m.handleOnlyAdmin(h, w, r)
	})

	router.HandleFunc(`/api/v1/label/{metric_name:[^/]*}/values`, func(w http.ResponseWriter, r *http.Request) {
		m.handleOnlyAdmin(h, w, r)
	})

	return router
}

func (m CFAuthMiddlewareProvider) authorized(sourceIds []string, c Oauth2ClientContext) bool {
	for _, sourceId := range sourceIds {
		if !m.logAuthorizer.IsAuthorized(sourceId, c.Token) {
			return false
		}
	}

	return true
}

func (m CFAuthMiddlewareProvider) handleOnlyAdmin(h http.Handler, w http.ResponseWriter, r *http.Request) {
	authToken := r.Header.Get("Authorization")
	userContext, err := m.oauth2Reader.Read(authToken)
	if err != nil {
		m.log.Error("failed to read from Oauth2 server", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if userContext.IsAdmin {
		w.WriteHeader(http.StatusOK)
		h.ServeHTTP(w, r)
	}

	w.WriteHeader(http.StatusNotFound)
}
