package metricstore_client

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	promhttp "github.com/prometheus/client_golang/api"
)

// Oauth2HTTPClient sets the "Authorization" header of any outgoing request.
// It gets a JWT from the configured Oauth2 server. It only gets a new JWT
// when a request comes back with a 401.
type Oauth2HTTPClient struct {
	c            *http.Client
	oauth2Addr   string
	client       string
	clientSecret string

	username     string
	userPassword string

	mu       sync.Mutex
	token    string
	endpoint *url.URL
}

// NewOauth2HTTPClient creates a new Oauth2HTTPClient.
func NewOauth2HTTPClient(addr, oauth2Addr, client, clientSecret string, opts ...Oauth2Option) *Oauth2HTTPClient {
	u, err := url.Parse(addr)
	if err != nil {
		return nil
	}
	u.Path = strings.TrimRight(u.Path, "/")

	c := &Oauth2HTTPClient{
		endpoint:     u,
		oauth2Addr:   oauth2Addr,
		client:       client,
		clientSecret: clientSecret,
		c: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	for _, o := range opts {
		o.configure(c)
	}

	return c
}

// Oauth2Option configures the Oauth2HTTPClient.
type Oauth2Option interface {
	configure(c *Oauth2HTTPClient)
}

// WithOauth2HTTPClient sets the HTTPClient for the Oauth2HTTPClient. It
// defaults to the same default as Client.
func WithOauth2HTTPClient(client *http.Client) Oauth2Option {
	return oauth2HTTPClientOptionFunc(func(c *Oauth2HTTPClient) {
		c.c = client
	})
}

// WithOauth2HTTPUser sets the username and password for user authentication.
func WithOauth2HTTPUser(username, password string) Oauth2Option {
	return oauth2HTTPClientOptionFunc(func(c *Oauth2HTTPClient) {
		c.username = username
		c.userPassword = password
	})
}

// oauth2HTTPClientOptionFunc enables a function to be a
// Oauth2Option.
type oauth2HTTPClientOptionFunc func(c *Oauth2HTTPClient)

// configure implements Oauth2Option.
func (f oauth2HTTPClientOptionFunc) configure(c *Oauth2HTTPClient) {
	f(c)
}

// Do implements HTTPClient. It adds the Authorization header to the request
// (unless the header already exists). If the token is expired, it will reach
// out the Oauth2 server and get a new one. The given error CAN be from the
// request to the Oauth2 server.
//
// Do modifies the given Request. It is invalid to use the same Request
// instance on multiple go-routines.
func (c *Oauth2HTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, promhttp.Warnings, error) {
	var body []byte
	done := make(chan struct{})

	if ctx != nil {
		req = req.WithContext(ctx)
	}

	if _, ok := req.Header["Authorization"]; ok {
		// Authorization Header is pre-populated, so just do the request.
		res, err := c.c.Do(req)

		defer func() {
			if res != nil {
				res.Body.Close()
			}
		}()

		if err != nil {
			return nil, nil, nil, err
		}

		go func() {
			body, err = ioutil.ReadAll(res.Body)
			close(done)
		}()

		select {
		case <-ctx.Done():
			<-done
			err = res.Body.Close()
			if err == nil {
				err = ctx.Err()
			}
		case <-done:
		}

		return res, body, nil, err
	}

	token, err := c.getToken()
	if err != nil {
		return nil, nil, nil, err
	}

	req.Header.Set("Authorization", token)

	res, err := c.c.Do(req)

	defer func() {
		if res != nil {
			res.Body.Close()
		}
	}()

	if err != nil {
		return nil, nil, nil, err
	}

	go func() {
		body, err = ioutil.ReadAll(res.Body)
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		err = res.Body.Close()
		if err == nil {
			err = ctx.Err()
		}
	case <-done:
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		c.token = ""
		return res, body, nil, nil
	}

	return res, body, nil, nil
}

func (c *Oauth2HTTPClient) URL(ep string, args map[string]string) *url.URL {
	p := path.Join(c.endpoint.Path, ep)

	for arg, val := range args {
		arg = ":" + arg
		p = strings.Replace(p, arg, val, -1)
	}

	u := *c.endpoint
	u.Path = p

	return &u
}

func (c *Oauth2HTTPClient) getToken() (string, error) {
	if c.username == "" {
		return c.getClientToken()
	}
	return c.getUserToken()
}

func (c *Oauth2HTTPClient) getClientToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" {
		return c.token, nil
	}

	v := make(url.Values)
	v.Set("client_id", c.client)
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

	req.URL.User = url.UserPassword(c.client, c.clientSecret)

	return c.doTokenRequest(req)
}

func (c *Oauth2HTTPClient) getUserToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" {
		return c.token, nil
	}

	v := make(url.Values)
	v.Set("client_id", c.client)
	v.Set("client_secret", c.clientSecret)
	v.Set("grant_type", "password")
	v.Set("username", c.username)
	v.Set("password", c.userPassword)

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

	return c.doTokenRequest(req)
}

func (c *Oauth2HTTPClient) doTokenRequest(req *http.Request) (string, error) {
	res, err := c.c.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code from Oauth2 server %d", res.StatusCode)
	}

	token := struct {
		TokenType   string `json:"token_type"`
		AccessToken string `json:"access_token"`
	}{}

	if err := json.NewDecoder(res.Body).Decode(&token); err != nil {
		return "", fmt.Errorf("failed to unmarshal response from Oauth2 server: %s", err)
	}

	c.token = fmt.Sprintf("%s %s", token.TokenType, token.AccessToken)
	return c.token, nil
}
