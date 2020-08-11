package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	jose "github.com/dvsekhvalnov/jose2go"
)

type UAAClient struct {
	httpClient             HTTPClient
	url                    *url.URL
	log                    *logger.Logger
	publicKeys             sync.Map
	minimumRefreshInterval time.Duration
	lastQueryTime          int64

	client       string
	clientSecret string
}

func NewUAAClient(
	uaaAddr string,
	httpClient HTTPClient,
	m metrics.Registrar, // TODO remove unused
	log *logger.Logger,
	opts ...UAAOption,
) *UAAClient {
	u, err := url.Parse(uaaAddr)
	if err != nil {
		log.Fatal("failed to parse UAA addr", err)
	}

	u.Path = "token_keys"

	uaa := &UAAClient{
		url:                    u,
		httpClient:             httpClient,
		log:                    log,
		publicKeys:             sync.Map{},
		minimumRefreshInterval: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(uaa)
	}

	return uaa
}

type UAAOption func(uaa *UAAClient)

func WithMinimumRefreshInterval(interval time.Duration) UAAOption {
	return func(uaa *UAAClient) {
		uaa.minimumRefreshInterval = interval
	}
}

func WithClientCredentials(client, secret string) UAAOption {
	return func(uaa *UAAClient) {
		uaa.client = client
		uaa.clientSecret = secret
	}
}

func (uaa *UAAClient) GetAuthHeader() (string, error) {
	if uaa.client == "" || uaa.clientSecret == "" {
		return "", fmt.Errorf("must provide client and secret to get auth header")
	}
	tokenType, accessToken, err := uaa.getAuthToken()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s", tokenType, accessToken), nil
}

type uaaTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func (uaa *UAAClient) getAuthToken() (tokenType, token string, err error) {
	bodyTemplate := `client_id=%s&client_secret=%s&grant_type=client_credentials&response_type=token`
	body := fmt.Sprintf(bodyTemplate, uaa.client, uaa.clientSecret)

	endpoint := fmt.Sprintf("https://%s:%s/oauth/token", uaa.url.Hostname(), uaa.url.Port())
	tokenRequest, err := http.NewRequest("POST", endpoint, strings.NewReader(body))
	if err != nil {
		return "", "", err
	}

	tokenRequest.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	tokenRequest.Header.Add("Accept", "application/json")

	dump, err := httputil.DumpRequestOut(tokenRequest, true)
	uaa.log.Debug("tokenRequest", logger.ByteString("tokenRequest", dump))

	responseBody, err := uaa.doRequest(tokenRequest)
	if err != nil {
		return "", "", err
	}

	uaaToken := &uaaTokenResponse{}
	err = json.Unmarshal(responseBody, uaaToken)
	if err != nil {
		return "", "", err
	}

	return uaaToken.TokenType, uaaToken.AccessToken, nil

}

func (uaa *UAAClient) doRequest(req *http.Request) ([]byte, error) {
	resp, err := uaa.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response code %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (uaa *UAAClient) RefreshTokenKeys() error {
	lastQueryTime := atomic.LoadInt64(&uaa.lastQueryTime)
	nextAllowedRefreshTime := time.Unix(0, lastQueryTime).Add(uaa.minimumRefreshInterval)
	if time.Now().Before(nextAllowedRefreshTime) {
		uaa.log.Info(
			"UAA TokenKey refresh throttled",
			logger.String("refresh interval", uaa.minimumRefreshInterval.String()),
			logger.String("retry in", time.Until(nextAllowedRefreshTime).Round(time.Millisecond).String()),
		)
		return nil
	}
	atomic.CompareAndSwapInt64(&uaa.lastQueryTime, lastQueryTime, time.Now().UnixNano())

	req, err := http.NewRequest("GET", uaa.url.String(), nil)
	if err != nil {
		uaa.log.Panic("failed to create request to UAA", logger.Error(err))
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := uaa.doRequest(req)
	if err != nil {
		return err
	}

	tokenKeys, err := unmarshalTokenKeys(resp)
	if err != nil {
		return err
	}

	currentKeyIds := make(map[string]struct{})

	uaa.publicKeys.Range(func(keyId, publicKey interface{}) bool {
		currentKeyIds[keyId.(string)] = struct{}{}
		return true
	})

	for _, tokenKey := range tokenKeys {
		if tokenKey.Value == "" {
			return fmt.Errorf("received an empty token key from UAA")
		}

		block, _ := pem.Decode([]byte(tokenKey.Value))
		if block == nil {
			return fmt.Errorf("failed to parse PEM block containing the public key")
		}

		publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("error parsing public key: %s", err)
		}

		publicKey, isRSAPublicKey := publicKeyInterface.(*rsa.PublicKey)
		if !isRSAPublicKey {
			return fmt.Errorf("did not get a valid RSA key from UAA: %s", err)
		}

		// always overwrite the key stored at keyId because:
		// if you manually delete the UAA signing key from Credhub, UAA will
		// generate a new key with the same (default) keyId, which is something
		// along the lines of `key-1`
		uaa.publicKeys.Store(tokenKey.KeyID, publicKey)

		// update list of previously-known keys so that we can prune them
		// if UAA no longer considers them valid
		delete(currentKeyIds, tokenKey.KeyID)
	}

	for keyID := range currentKeyIds {
		uaa.publicKeys.Delete(keyID)
	}

	return nil
}

type AlgorithmError struct {
	Alg string
}

func (e AlgorithmError) Error() string {
	return fmt.Sprintf("unsupported algorithm: %s", e.Alg)
}

type UnknownTokenKeyError struct {
	Kid string
}

func (e UnknownTokenKeyError) Error() string {
	return fmt.Sprintf("using unknown token key: %s", e.Kid)
}

func (uaa *UAAClient) Read(token string) (Oauth2ClientContext, error) {
	if token == "" {
		return Oauth2ClientContext{}, errors.New("missing token")
	}

	payload, _, err := jose.Decode(trimBearer(token), func(headers map[string]interface{}, payload string) interface{} {
		if headers["alg"] != "RS256" {
			return AlgorithmError{Alg: headers["alg"].(string)}
		}

		keyID := headers["kid"].(string)

		publicKey, err := uaa.loadOrFetchPublicKey(keyID)
		if err != nil {
			return err
		}

		return publicKey
	})

	if err != nil {
		switch err.(type) {
		case AlgorithmError, UnknownTokenKeyError:
			// no-op
		default:
			// we're specifically trying to catch "crypto/rsa: verification error",
			// which generally means we've tried to decode a token with the
			// wrong private key. just in case UAA has rolled the key, but
			// kept the same keyId, let's renew our keys.
			go uaa.RefreshTokenKeys()
		}

		return Oauth2ClientContext{}, fmt.Errorf("failed to decode token: %s", err.Error())
	}

	decodedToken, err := decodeToken(strings.NewReader(payload))
	if err != nil {
		return Oauth2ClientContext{}, fmt.Errorf("failed to unmarshal token: %s", err.Error())
	}

	if time.Now().After(decodedToken.ExpTime) {
		return Oauth2ClientContext{}, fmt.Errorf("token is expired, exp = %s", decodedToken.ExpTime)
	}

	var isAdmin bool
	for _, scope := range decodedToken.Scope {
		if scope == "doppler.firehose" || scope == "logs.admin" {
			isAdmin = true
		}
	}

	return Oauth2ClientContext{
		IsAdmin:   isAdmin,
		Token:     token,
		ExpiresAt: decodedToken.ExpTime,
	}, err
}

func (uaa *UAAClient) loadOrFetchPublicKey(keyID string) (*rsa.PublicKey, error) {
	publicKey, ok := uaa.publicKeys.Load(keyID)
	if ok {
		return (publicKey.(*rsa.PublicKey)), nil
	}

	uaa.RefreshTokenKeys()

	publicKey, ok = uaa.publicKeys.Load(keyID)
	if ok {
		return (publicKey.(*rsa.PublicKey)), nil
	}

	return nil, UnknownTokenKeyError{Kid: keyID}
}

var bearerRE = regexp.MustCompile(`(?i)^bearer\s+`)

func trimBearer(authToken string) string {
	return bearerRE.ReplaceAllString(authToken, "")
}

// TODO: move key processing to a method of tokenKey
type tokenKey struct {
	KeyID string `json:"kid"`
	Value string `json:"value"`
}

type tokenKeys struct {
	Keys []tokenKey `json:"keys"`
}

func unmarshalTokenKeys(r []byte) ([]tokenKey, error) {
	var dtks tokenKeys
	if err := json.Unmarshal(r, &dtks); err != nil {
		return []tokenKey{}, fmt.Errorf("unable to decode json token keys from UAA: %s", err)
	}

	return dtks.Keys, nil
}

type decodedToken struct {
	Value   string    `json:"value"`
	Scope   []string  `json:"scope"`
	Exp     float64   `json:"exp"`
	ExpTime time.Time `json:"-"`
}

func decodeToken(r io.Reader) (decodedToken, error) {
	var dt decodedToken
	if err := json.NewDecoder(r).Decode(&dt); err != nil {
		return decodedToken{}, fmt.Errorf("unable to decode json token from UAA: %s", err)
	}

	dt.ExpTime = time.Unix(int64(dt.Exp), 0)

	return dt, nil
}
