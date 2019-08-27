package auth

import "net/http"

// MetricRegistrar is used to update values of metrics.
type MetricRegistrar interface {
	Set(name string, value float64)
	// Add(name string, delta float64)
	// Inc(name string)
}

type Oauth2ClientReader interface {
	Read(token string) (Oauth2ClientContext, error)
}

type QueryParser interface {
	ExtractSourceIds(query string) ([]string, error)
}

type LogAuthorizer interface {
	IsAuthorized(sourceId string, clientToken string) bool
	AvailableSourceIDs(token string) []string
}

type AccessLogger interface {
	LogAccess(req *http.Request, host, port string) error
}

type HTTPClient interface {
	Do(r *http.Request) (*http.Response, error)
}
