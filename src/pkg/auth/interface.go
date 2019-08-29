package auth

import "net/http"

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
