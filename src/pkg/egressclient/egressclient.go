package egressclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	prom_api_client "github.com/prometheus/client_golang/api"
	prom_versioned_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
)

// EgressClient sets the "Authorization" header of any outgoing request.
// It gets a JWT from the configured Oauth2 server. It only gets a new JWT
// when a request comes back with a 401.
type EgressClient struct {
	mu sync.Mutex

	client       prom_api_client.Client
	oauth2Addr   string
	clientID     string
	clientSecret string

	token string
}

func NewEgressClient(httpAddr, uaaAddr, uaaClientId, uaaClientSecret string) (prom_versioned_api_client.API, error) {
	promAPIClient, err := prom_api_client.NewClient(prom_api_client.Config{
		Address: httpAddr,
	})
	if err != nil {
		return nil, err
	}

	egressClient := &EgressClient{
		oauth2Addr:   uaaAddr,
		clientID:     uaaClientId,
		clientSecret: uaaClientSecret,
		client:       promAPIClient,
	}

	promVersionedAPIClient := prom_versioned_api_client.NewAPI(egressClient)

	return promVersionedAPIClient, nil
}

// Do implements HTTPClient. It adds the Authorization header to the request
// (unless the header already exists). If the token is expired, it will reach
// out the Oauth2 server and get a new one. The given error CAN be from the
// request to the Oauth2 server.
//
// Do modifies the given Request. It is invalid to use the same Request
// instance on multiple go-routines.
func (c *EgressClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	req.Header.Add("Accept-Encoding", "text/plain")

	if _, ok := req.Header["Authorization"]; ok {
		// Authorization Header is pre-populated, so just do the request.
		return c.client.Do(ctx, req)
	}

	token, err := c.getToken()
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Authorization", token)

	res, body, err := c.client.Do(ctx, req)

	if err != nil {
		c.token = ""
		return res, body, err
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		c.token = ""
		return res, body, nil
	}

	return res, body, nil
}

func (c *EgressClient) URL(ep string, args map[string]string) *url.URL {
	return c.client.URL(ep, args)
}

func (c *EgressClient) getToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" {
		return c.token, nil
	}

	v := make(url.Values)
	v.Set("client_id", c.clientID)
	v.Set("grant_type", "client_credentials")

	req, err := http.NewRequest(
		"POST",
		c.oauth2Addr,
		strings.NewReader(v.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.URL.Path = "/oauth/token"

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	req.URL.User = url.UserPassword(c.clientID, c.clientSecret)

	return c.doTokenRequest(req)
}

func (c *EgressClient) doTokenRequest(req *http.Request) (string, error) {
	res, body, err := c.client.Do(context.Background(), req)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code from Oauth2 server %d", res.StatusCode)
	}

	token := struct {
		TokenType   string `json:"token_type"`
		AccessToken string `json:"access_token"`
	}{}

	if err := json.Unmarshal(body, &token); err != nil {
		return "", fmt.Errorf("failed to unmarshal response from Oauth2 server: %s", err)
	}

	c.token = fmt.Sprintf("%s %s", token.TokenType, token.AccessToken)
	return c.token, nil
}
