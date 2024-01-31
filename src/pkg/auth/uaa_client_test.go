package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"

	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"net/http"

	jose "github.com/dvsekhvalnov/jose2go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("UAAClient", func() {
	Context("ReadTCP()", func() {
		var tc *UAATestContext
		BeforeEach(func() {
			tc = uaaSetup()
			tc.PrimePublicKeyCache()
		})

		It("only accepts tokens that are signed with RS256", func() {
			payload := tc.BuildValidPayload("doppler.firehose")
			token := tc.CreateUnsignedToken(payload)
			_, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to decode token: unsupported algorithm: none"))
		})

		It("returns IsAdmin == true when scopes include doppler.firehose", func() {
			payload := tc.BuildValidPayload("doppler.firehose")
			token := tc.CreateSignedToken(payload)
			c, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Token).To(Equal(withBearer(token)))
			Expect(c.IsAdmin).To(BeTrue())
		})

		It("returns IsAdmin == true when scopes include logs.admin", func() {
			payload := tc.BuildValidPayload("logs.admin")
			token := tc.CreateSignedToken(payload)
			c, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Token).To(Equal(withBearer(token)))
			Expect(c.IsAdmin).To(BeTrue())
		})

		It("returns IsAdmin == false when scopes include neither logs.admin nor doppler.firehose", func() {
			payload := tc.BuildValidPayload("foo.bar")
			token := tc.CreateSignedToken(payload)
			c, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Token).To(Equal(withBearer(token)))
			Expect(c.IsAdmin).To(BeFalse())
		})

		It("returns context with correct ExpiresAt", func() {
			t := time.Now().Add(time.Hour).Truncate(time.Second)
			payload := fmt.Sprintf(`{"scope":["logs.admin"], "exp":%d}`, t.Unix())
			token := tc.CreateSignedToken(payload)
			c, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Token).To(Equal(withBearer(token)))
			Expect(c.ExpiresAt).To(Equal(t))
		})

		It("does offline token validation", func() {
			initialRequestCount := len(tc.httpClient.requests)
			payload := tc.BuildValidPayload("logs.admin")
			token := tc.CreateSignedToken(payload)
			tc.uaaClient.Read(withBearer(token))
			tc.uaaClient.Read(withBearer(token))
			Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount))
		})

		It("does not allow use of an expired token", func() {
			tc.GenerateSingleTokenKeyResponse()
			tc.uaaClient.RefreshTokenKeys()
			expiredPayload := tc.BuildExpiredPayload("logs.Admin")
			expiredToken := tc.CreateSignedToken(expiredPayload)
			_, err := tc.uaaClient.Read(withBearer(expiredToken))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("token is expired"))
		})

		It("returns an error when token is blank", func() {
			_, err := tc.uaaClient.Read("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("missing token"))
		})

		Context("when a token is signed with a private key that is unknown", func() {
			It("validates the token successfully when the matching public key can be retrieved from UAA", func() {
				initialRequestCount := len(tc.httpClient.requests)
				newPrivateKey := generateLegitTokenKey("testKey2")
				tc.AddPrivateKeyToUAATokenKeyResponse(newPrivateKey)
				payload := tc.BuildValidPayload("logs.admin")
				token := tc.CreateSignedTokenUsingPrivateKey(payload, newPrivateKey)
				c, err := tc.uaaClient.Read(withBearer(token))
				Expect(err).ToNot(HaveOccurred())
				Expect(c.Token).To(Equal(withBearer(token)))
				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))
			})

			It("returns an error when the matching public key cannot be retrieved from UAA", func() {
				initialRequestCount := len(tc.httpClient.requests)
				newPrivateKey := generateLegitTokenKey("testKey2")
				payload := tc.BuildValidPayload("logs.admin")
				token := tc.CreateSignedTokenUsingPrivateKey(payload, newPrivateKey)
				_, err := tc.uaaClient.Read(withBearer(token))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to decode token: using unknown token key"))
				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))
			})

			It("returns an error when given a token signed by an public key that was purged from UAA", func() {
				initialRequestCount := len(tc.httpClient.requests)
				payload := tc.BuildValidPayload("logs.admin")
				tokenSignedWithExpiredPrivateKey := tc.CreateSignedToken(payload)
				newAndOnlyPrivateKey := generateLegitTokenKey("testKey2")
				tc.MockUAATokenKeyResponseUsingPrivateKey(newAndOnlyPrivateKey)
				payload = tc.BuildValidPayload("logs.admin")
				tokenSignedWithNewPrivateKey := tc.CreateSignedTokenUsingPrivateKey(payload, newAndOnlyPrivateKey)
				_, err := tc.uaaClient.Read(withBearer(tokenSignedWithNewPrivateKey))
				Expect(err).ToNot(HaveOccurred())
				_, err = tc.uaaClient.Read(withBearer(tokenSignedWithExpiredPrivateKey))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to decode token: using unknown token key"))
				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 2))
			})

			It("continues to accept previously signed tokens when retrieving public keys from UAA fails", func() {
				initialRequestCount := len(tc.httpClient.requests)
				payload := tc.BuildValidPayload("logs.admin")
				toBeExpiredToken := tc.CreateSignedToken(payload)
				_, err := tc.uaaClient.Read(withBearer(toBeExpiredToken))
				Expect(err).ToNot(HaveOccurred())
				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount))
				newTokenKey := generateLegitTokenKey("testKey2")
				refreshedToken := tc.CreateSignedTokenUsingPrivateKey(payload, newTokenKey)
				newTokenKey.publicKey = "corrupted public key"
				tc.MockUAATokenKeyResponseUsingPrivateKey(newTokenKey)
				_, err = tc.uaaClient.Read(withBearer(refreshedToken))
				Expect(err).To(HaveOccurred())
				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))
				_, err = tc.uaaClient.Read(withBearer(toBeExpiredToken))
				Expect(err).ToNot(HaveOccurred())
				Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))
			})
		})

		It("returns an error when given a token signed by an unknown but valid key", func() {
			initialRequestCount := len(tc.httpClient.requests)
			unknownPrivateKey := generateLegitTokenKey("testKey99")
			payload := tc.BuildValidPayload("logs.admin")
			tokenSignedWithUnknownPrivateKey := tc.CreateSignedTokenUsingPrivateKey(payload, unknownPrivateKey)
			_, err := tc.uaaClient.Read(withBearer(tokenSignedWithUnknownPrivateKey))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to decode token: using unknown token key"))
			Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))
		})

		It("returns an error when the provided token cannot be decoded", func() {
			_, err := tc.uaaClient.Read("any-old-token")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to decode token"))
		})

		DescribeTable("handling the Bearer prefix in the Authorization header",
			func(prefix string) {
				payload := tc.BuildValidPayload("foo.bar")
				token := tc.CreateSignedToken(payload)
				c, err := tc.uaaClient.Read(withBearer(token))
				Expect(err).ToNot(HaveOccurred())
				Expect(c.Token).To(Equal(withBearer(token)))
			},
			Entry("Standard 'Bearer' prefix", "Bearer "),
			Entry("Non-Standard 'bearer' prefix", "bearer "),
			Entry("No prefix", ""),
		)
	})

	Context("RefreshTokenKeys()", func() {
		It("handles concurrent refreshes", func() {
			tc := uaaSetup()
			tc.GenerateSingleTokenKeyResponse()
			tc.uaaClient.RefreshTokenKeys()
			payload := tc.BuildValidPayload("logs.admin")
			token := tc.CreateSignedToken(payload)
			numRequests := len(tc.httpClient.requests)
			var wg sync.WaitGroup
			for n := 0; n < 4; n++ {
				wg.Add(1)
				go func(wg *sync.WaitGroup) {
					tc.uaaClient.Read(withBearer(token))
					tc.uaaClient.RefreshTokenKeys()
					tc.uaaClient.Read(withBearer(token))
					wg.Done()
				}(&wg)
			}
			wg.Wait()
			Expect(len(tc.httpClient.requests)).To(Equal(numRequests + 4))
		})

		It("calls UAA correctly", func() {
			tc := uaaSetup()
			tc.GenerateSingleTokenKeyResponse()
			tc.uaaClient.RefreshTokenKeys()
			r := tc.httpClient.requests[0]
			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))
			Expect(r.URL.Path).To(Equal("/token_keys"))
			// confirm that we're not using any authentication
			_, _, ok := r.BasicAuth()
			Expect(ok).To(BeFalse())
			Expect(r.Body).To(BeNil())
		})

		It("returns an error when UAA cannot be reached", func() {
			tc := uaaSetup()
			tc.httpClient.resps = []response{{
				err: errors.New("error!"),
			}}
			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when UAA returns a non-200 response", func() {
			tc := uaaSetup()
			tc.httpClient.resps = []response{{
				body:   []byte{},
				status: http.StatusUnauthorized,
			}}
			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the response from the UAA is malformed", func() {
			tc := uaaSetup()
			tc.httpClient.resps = []response{{
				body:   []byte("garbage"),
				status: http.StatusOK,
			}}
			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the response from the UAA has an empty key", func() {
			tc := uaaSetup()
			tc.GenerateEmptyTokenKeyResponse()
			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the response from the UAA has an unparsable PEM format", func() {
			tc := uaaSetup()
			tc.GenerateTokenKeyResponseWithInvalidPEM()
			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to parse PEM block containing the public key"))
		})

		It("returns an error when the response from the UAA has an invalid key format", func() {
			tc := uaaSetup()
			tc.GenerateTokenKeyResponseWithInvalidKey()
			err := tc.uaaClient.RefreshTokenKeys()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error parsing public key"))
		})

		It("overwrites a pre-existing keyId with the new key", func() {
			tc := uaaSetup()
			tc.PrimePublicKeyCache()
			payload := tc.BuildValidPayload("doppler.firehose")
			token := tc.CreateSignedToken(payload)
			_, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).NotTo(HaveOccurred())
			tokenKey := generateLegitTokenKey("testKey1")
			tc.GenerateTokenKeyResponse([]mockTokenKey{tokenKey})
			tc.uaaClient.RefreshTokenKeys()
			_, err = tc.uaaClient.Read(withBearer(token))
			Expect(err).To(HaveOccurred())
			newToken := tc.CreateSignedTokenUsingPrivateKey(payload, tokenKey)
			_, err = tc.uaaClient.Read(withBearer(newToken))
			Expect(err).NotTo(HaveOccurred())
		})

		It("overwrites a pre-existing keyId with the new key", func() {
			tc := uaaSetup()
			tc.PrimePublicKeyCache()
			payload := tc.BuildValidPayload("doppler.firehose")
			token := tc.CreateSignedToken(payload)
			_, err := tc.uaaClient.Read(withBearer(token))
			Expect(err).NotTo(HaveOccurred())
			tokenKey := generateLegitTokenKey("testKey1")
			tc.GenerateTokenKeyResponse([]mockTokenKey{tokenKey})
			newToken := tc.CreateSignedTokenUsingPrivateKey(payload, tokenKey)
			Eventually(func() bool {
				_, err = tc.uaaClient.Read(withBearer(newToken))
				return err == nil
			}).Should(BeTrue())
			_, err = tc.uaaClient.Read(withBearer(token))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("crypto/rsa: verification error"))
		})

		It("rate limits UAA TokenKey refreshes", func() {
			tc := uaaSetup(auth.WithMinimumRefreshInterval(200 * time.Millisecond))
			tc.GenerateSingleTokenKeyResponse()
			initialRequestCount := len(tc.httpClient.requests)
			tc.uaaClient.RefreshTokenKeys()
			Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))
			time.Sleep(100 * time.Millisecond)
			tc.uaaClient.RefreshTokenKeys()
			Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 1))
			time.Sleep(101 * time.Millisecond)
			tc.uaaClient.RefreshTokenKeys()
			Expect(len(tc.httpClient.requests)).To(Equal(initialRequestCount + 2))
		})
	})
})

type mockTokenKey struct {
	privateKey *rsa.PrivateKey
	publicKey  string
	keyId      string
}

func generateLegitTokenKey(keyId string) mockTokenKey {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	publicKey := publicKeyPEMToString(privateKey)
	return mockTokenKey{
		privateKey: privateKey,
		publicKey:  publicKey,
		keyId:      keyId,
	}
}

func uaaSetup(opts ...auth.UAAOption) *UAATestContext {
	httpClient := newSpyHTTPClient()
	tokenKey := generateLegitTokenKey("testKey1")
	// default the minimumRefreshInterval in tests to 0, but make sure we
	// apply user-provided options afterwards
	opts = append([]auth.UAAOption{auth.WithMinimumRefreshInterval(0)}, opts...)
	uaaClient := auth.NewUAAClient(
		"https://uaa.com",
		httpClient,
		testing.NewSpyMetricRegistrar(),
		logger.NewTestLogger(GinkgoWriter),
		opts...,
	)
	return &UAATestContext{
		uaaClient:   uaaClient,
		httpClient:  httpClient,
		privateKeys: []mockTokenKey{tokenKey},
	}
}

type UAATestContext struct {
	uaaClient   *auth.UAAClient
	httpClient  *spyHTTPClient
	privateKeys []mockTokenKey
}

func (tc *UAATestContext) PrimePublicKeyCache() {
	tc.GenerateSingleTokenKeyResponse()
	err := tc.uaaClient.RefreshTokenKeys()
	Expect(err).ToNot(HaveOccurred())
}

func (tc *UAATestContext) BuildValidPayload(scope string) string {
	t := time.Now().Add(time.Hour).Truncate(time.Second)
	payload := fmt.Sprintf(`{"scope":["%s"], "exp":%d}`, scope, t.Unix())
	return payload
}

func (tc *UAATestContext) BuildExpiredPayload(scope string) string {
	t := time.Now().Add(-time.Minute)
	payload := fmt.Sprintf(`{"scope":["%s"], "exp":%d}`, scope, t.Unix())
	return payload
}

func (tc *UAATestContext) GenerateTokenKeyResponse(mockTokenKeys []mockTokenKey) {
	var tokenKeys []map[string]string
	for _, mockPrivateKey := range mockTokenKeys {
		tokenKey := map[string]string{
			"kty":   "RSA",
			"use":   "sig",
			"kid":   mockPrivateKey.keyId,
			"alg":   "RS256",
			"value": mockPrivateKey.publicKey,
		}
		tokenKeys = append(tokenKeys, tokenKey)
	}
	data, err := json.Marshal(map[string][]map[string]string{
		"keys": tokenKeys,
	})
	Expect(err).ToNot(HaveOccurred())
	tc.httpClient.resps = []response{{
		body:   data,
		status: http.StatusOK,
	}}
}

func (tc *UAATestContext) GenerateSingleTokenKeyResponse() {
	tc.GenerateTokenKeyResponse(
		[]mockTokenKey{
			tc.privateKeys[0],
		},
	)
}

func (tc *UAATestContext) MockUAATokenKeyResponseUsingPrivateKey(tokenKey mockTokenKey) {
	tc.GenerateTokenKeyResponse(
		[]mockTokenKey{
			tokenKey,
		},
	)
}

func (tc *UAATestContext) AddPrivateKeyToUAATokenKeyResponse(tokenKey mockTokenKey) {
	tc.GenerateTokenKeyResponse(
		[]mockTokenKey{
			tokenKey,
			tc.privateKeys[0],
		},
	)
}

func (tc *UAATestContext) GenerateEmptyTokenKeyResponse() {
	tc.GenerateTokenKeyResponse(
		[]mockTokenKey{
			{publicKey: "", keyId: ""},
		},
	)
}

func (tc *UAATestContext) GenerateTokenKeyResponseWithInvalidPEM() {
	tc.GenerateTokenKeyResponse(
		[]mockTokenKey{
			{publicKey: "-- BEGIN SOMETHING --\nNOTVALIDPEM\n-- END SOMETHING --\n", keyId: ""},
		},
	)
}

func (tc *UAATestContext) GenerateTokenKeyResponseWithInvalidKey() {
	pem := publicKeyPEMToString(tc.privateKeys[0].privateKey)
	tc.GenerateTokenKeyResponse(
		[]mockTokenKey{
			{publicKey: strings.Replace(pem, "MIIB", "XXXX", 1), keyId: ""},
		},
	)
}

func (tc *UAATestContext) CreateSignedToken(payload string) string {
	tokenKey := tc.privateKeys[0]
	token, err := jose.Sign(payload, jose.RS256, tokenKey.privateKey, jose.Header("kid", tokenKey.keyId))
	Expect(err).ToNot(HaveOccurred())
	return token
}

func (tc *UAATestContext) CreateSignedTokenUsingPrivateKey(payload string, tokenKey mockTokenKey) string {
	token, err := jose.Sign(payload, jose.RS256, tokenKey.privateKey, jose.Header("kid", tokenKey.keyId))
	Expect(err).ToNot(HaveOccurred())
	return token
}

func (tc *UAATestContext) CreateUnsignedToken(payload string) string {
	token, err := jose.Sign(payload, jose.NONE, nil)
	Expect(err).ToNot(HaveOccurred())
	return token
}

type spyHTTPClient struct {
	mu       sync.Mutex
	requests []*http.Request
	resps    []response
	tokens   []string
}

type response struct {
	status int
	err    error
	body   []byte
}

func newSpyHTTPClient() *spyHTTPClient {
	return &spyHTTPClient{}
}

func (s *spyHTTPClient) Do(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r)
	s.tokens = append(s.tokens, r.Header.Get("Authorization"))
	if len(s.resps) == 0 {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader(nil)),
		}, nil
	}
	result := s.resps[0]
	s.resps = s.resps[1:]
	resp := http.Response{
		StatusCode: result.status,
		Body:       ioutil.NopCloser(bytes.NewReader(result.body)),
	}
	if result.err != nil {
		return nil, result.err
	}
	return &resp, nil
}

func publicKeyExponentToString(privateKey *rsa.PrivateKey) string {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(privateKey.PublicKey.E))
	return base64.StdEncoding.EncodeToString(b[0:3])
}

func publicKeyPEMToString(privateKey *rsa.PrivateKey) string {
	encodedKey, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	Expect(err).ToNot(HaveOccurred())
	var pemKey = &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: encodedKey,
	}
	return string(pem.EncodeToMemory(pemKey))
}

func publicKeyModulusToString(privateKey *rsa.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(privateKey.PublicKey.N.Bytes())
}

func withBearer(token string) string {
	return fmt.Sprintf("Bearer %s", token)
}
