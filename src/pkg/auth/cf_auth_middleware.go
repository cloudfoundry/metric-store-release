package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/golang/protobuf/jsonpb"
	"github.com/gorilla/mux"
)

type CFAuthMiddlewareProvider struct {
	log     *logger.Logger
	metrics debug.MetricRegistrar

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
	metrics debug.MetricRegistrar,
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

type queryError struct {
	promqlErrorBody
	Data     interface{} `json:"data"`
	Warnings []string    `json:"warnings"`
}

func (m CFAuthMiddlewareProvider) Middleware(h http.Handler) http.Handler {
	router := mux.NewRouter()

	router.HandleFunc("/api/v1/{subpath:query|query_range}", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		authToken := r.Header.Get("Authorization")
		if authToken == "" {
			m.writeQueryError(w, http.StatusNotFound, "missing auth token")
			return
		}

		userContext, err := m.oauth2Reader.Read(authToken)
		if err != nil {
			m.log.Error("failed to read from Oauth2 server", err)
			m.writeQueryError(w, http.StatusNotFound, "failed to read from Oauth2 server")
			return
		}

		if !userContext.IsAdmin {
			query := r.URL.Query().Get("query")
			relevantSourceIds, err := m.queryParser.ExtractSourceIds(query)
			if err != nil {
				m.writeQueryError(w, http.StatusUnprocessableEntity, "query parse error: %s", err)
				return
			}
			if !m.authorized(relevantSourceIds, userContext) {
				m.writeQueryError(w, http.StatusUnauthorized, "there are no matching source IDs for your query")
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

	router.HandleFunc("/api/v1/rules", func(w http.ResponseWriter, r *http.Request) {
		m.handleOnlyAdmin(h, w, r)
	})

	router.HandleFunc("/api/v1/alerts", func(w http.ResponseWriter, r *http.Request) {
		m.handleOnlyAdmin(h, w, r)
	})

	router.HandleFunc(`/api/v1/label/{metric_name:[^/]*}/values`, func(w http.ResponseWriter, r *http.Request) {
		m.handleOnlyAdmin(h, w, r)
	})

	router.HandleFunc("/health", h.ServeHTTP)

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
		m.writeQueryError(w, http.StatusNotFound, "failed to read from Oauth2 server")
		return
	}

	if userContext.IsAdmin {
		h.ServeHTTP(w, r)
		return
	}

	m.writeQueryError(w, http.StatusNotFound, "")
}

func (m CFAuthMiddlewareProvider) writeQueryError(w http.ResponseWriter, statusCode int, errFmt string, a ...interface{}) {
	e := queryError{
		promqlErrorBody: promqlErrorBody{
			Status:    "error",
			ErrorType: http.StatusText(statusCode),
			Error:     fmt.Sprintf(errFmt, a...),
		},
		Warnings: []string{},
	}

	result, err := json.Marshal(e)
	if err != nil {
		m.log.Error("query error message marshalling failure", err)
		result = []byte(e.Error)
	}
	http.Error(w, string(result), statusCode)
}
