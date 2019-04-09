package auth

import (
	"fmt"
	"net/http"
	"time"

	"log"

	"github.com/golang/protobuf/jsonpb"
	"github.com/gorilla/mux"
)

type CFAuthMiddlewareProvider struct {
	oauth2Reader  Oauth2ClientReader
	logAuthorizer LogAuthorizer
	queryParser   QueryParser
	marshaller    jsonpb.Marshaler
	metrics       authMetrics
}

type Oauth2ClientContext struct {
	IsAdmin   bool
	Token     string
	ExpiresAt time.Time
}

type Oauth2ClientReader interface {
	Read(token string) (Oauth2ClientContext, error)
}

type QueryParser interface {
	ExtractSourceIds(query string) ([]string, error)
}

type LogAuthorizer interface {
	IsAuthorized(sourceId string, clientToken string) bool
	AvailableSourceIds(token string) []string
}

type authMetrics struct {
	setTotalReadTime  func(float64)
	setTotalQueryTime func(float64)
}

func NewCFAuthMiddlewareProvider(
	oauth2Reader Oauth2ClientReader,
	logAuthorizer LogAuthorizer,
	queryParser QueryParser,
	metrics Metrics,
) CFAuthMiddlewareProvider {
	return CFAuthMiddlewareProvider{
		oauth2Reader:  oauth2Reader,
		logAuthorizer: logAuthorizer,
		queryParser:   queryParser,
		metrics: authMetrics{
			setTotalReadTime:  metrics.NewGauge("cf_auth_proxy_total_read_time", "nanoseconds"),
			setTotalQueryTime: metrics.NewGauge("cf_auth_proxy_total_query_time", "nanoseconds"),
		},
	}
}

type promqlErrorBody struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

func (m CFAuthMiddlewareProvider) Middleware(h http.Handler) http.Handler {
	router := mux.NewRouter()

	router.HandleFunc("/api/v1/read/{name}", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		_, ok := mux.Vars(r)["name"]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		authToken := r.Header.Get("Authorization")
		if authToken == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		userContext, err := m.oauth2Reader.Read(authToken)
		if err != nil {
			log.Printf("failed to read from Oauth2 server: %s", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if !userContext.IsAdmin {
			// /api/v1/read is intended only for node-to-node communication
			// it does not provide a non-admin auth model
			w.WriteHeader(http.StatusNotFound)
			return
		}

		h.ServeHTTP(w, r)

		totalReadTime := time.Since(start).Nanoseconds()
		m.metrics.setTotalReadTime(float64(totalReadTime))
	})

	router.HandleFunc("/api/v1/{subpath:query|query_range}", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		authToken := r.Header.Get("Authorization")
		if authToken == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		userContext, err := m.oauth2Reader.Read(authToken)
		if err != nil {
			log.Printf("failed to read from Oauth2 server: %s", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if !userContext.IsAdmin {
			query := r.URL.Query().Get("query")
			relevantSourceIds, err := m.queryParser.ExtractSourceIds(query)
			if err != nil || !m.authorized(relevantSourceIds, userContext) {
				http.Error(w, fmt.Sprintf("query parse error: %s", err), http.StatusUnprocessableEntity)
				return
			}
		}

		h.ServeHTTP(w, r)

		totalQueryTime := time.Since(start).Nanoseconds()
		m.metrics.setTotalQueryTime(float64(totalQueryTime))
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
		log.Printf("failed to read from Oauth2 server: %s", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if userContext.IsAdmin {
		w.WriteHeader(http.StatusOK)
		h.ServeHTTP(w, r)
	}

	w.WriteHeader(http.StatusNotFound)
}
