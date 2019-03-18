package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Metrics interface {
	NewGauge(name, unit string) func(value float64)
}

type HTTPClient interface {
	Do(r *http.Request) (*http.Response, error)
}

type UAAClient struct {
	httpClient   HTTPClient
	uaa          *url.URL
	client       string
	clientSecret string
	tokenCache   map[string]uaaResponse
	storeLatency func(float64)
}

func NewUAAClient(
	uaaAddr string,
	client string,
	clientSecret string,
	httpClient HTTPClient,
	m Metrics,
	log *log.Logger,
) *UAAClient {
	u, err := url.Parse(uaaAddr)
	if err != nil {
		log.Fatalf("failed to parse UAA addr: %s", err)
	}

	u.Path = "check_token"

	return &UAAClient{
		uaa:          u,
		client:       client,
		clientSecret: clientSecret,
		httpClient:   httpClient,
		storeLatency: m.NewGauge("cf_auth_proxy_last_uaa_latency", "nanoseconds"),
		tokenCache:   make(map[string]uaaResponse),
	}
}

func (c *UAAClient) Read(token string) (Oauth2ClientContext, error) {
	if token == "" {
		return Oauth2ClientContext{}, errors.New("missing token")
	}

	uaaR, ok := c.tokenCache[token]
	if ok && time.Now().Before(uaaR.ExpTime) {
		return Oauth2ClientContext{
			IsAdmin: c.hasDopplerScope(uaaR),
			Token:   token,
		}, nil
	}

	form := url.Values{
		"token": {trimBearer(token)},
	}

	req, err := http.NewRequest("POST", c.uaa.String(), strings.NewReader(form.Encode()))
	if err != nil {
		log.Printf("failed to create UAA request: %s", err)
		return Oauth2ClientContext{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.client, c.clientSecret)

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	c.storeLatency(float64(time.Since(start)))
	if err != nil {
		log.Printf("UAA request failed: %s", err)
		return Oauth2ClientContext{}, err
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return Oauth2ClientContext{}, fmt.Errorf("Non-200 status code received from UAA")
	}

	uaaR, err = c.parseResponse(resp.Body)
	if err != nil {
		log.Printf("failed to parse UAA response body: %s", err)
		return Oauth2ClientContext{}, err
	}

	c.tokenCache[token] = uaaR

	return Oauth2ClientContext{
		IsAdmin: resp.StatusCode == http.StatusOK && c.hasDopplerScope(uaaR),
		Token:   token,
	}, nil
}

func trimBearer(authToken string) string {
	return strings.TrimSpace(strings.TrimPrefix(authToken, "bearer"))
}

type uaaResponse struct {
	Scopes  []string  `json:"scope"`
	Exp     float64   `json:"exp"`
	ExpTime time.Time `json:"-"`
}

func (c *UAAClient) hasDopplerScope(r uaaResponse) bool {
	for _, scope := range r.Scopes {
		if scope == "doppler.firehose" || scope == "logs.admin" {
			return true
		}
	}

	return false
}

func (c *UAAClient) parseResponse(r io.Reader) (uaaResponse, error) {
	var resp uaaResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		log.Printf("unable to decode json response from UAA: %s", err)
		return uaaResponse{}, err
	}

	resp.ExpTime = time.Unix(0, int64(resp.Exp*1e9))

	return resp, nil
}
