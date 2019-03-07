package auth_test

import (
	"log"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store/pkg/auth"
	"github.com/cloudfoundry/metric-store/pkg/testing"

	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UAAClient", func() {
	var (
		client     *auth.UAAClient
		httpClient *spyHTTPClient
		metrics    *testing.SpyMetrics
	)

	BeforeEach(func() {
		httpClient = newSpyHTTPClient()
		metrics = testing.NewSpyMetrics()
		client = auth.NewUAAClient(
			"https://uaa.com",
			"some-client",
			"some-client-secret",
			httpClient,
			metrics,
			log.New(ioutil.Discard, "", 0),
		)
	})

	It("returns IsAdmin when scopes include doppler.firehose with correct clientID and UserID", func() {
		data, err := json.Marshal(map[string]interface{}{
			"scope": []string{"doppler.firehose"},
		})
		Expect(err).ToNot(HaveOccurred())

		httpClient.resps = []response{{
			body:   data,
			status: http.StatusOK,
		}}

		c, err := client.Read("valid-token")
		Expect(err).ToNot(HaveOccurred())
		Expect(c.IsAdmin).To(BeTrue())
		Expect(c.Token).To(Equal("valid-token"))
	})

	It("returns IsAdmin when scopes include logs.admin with correct clientID and UserID", func() {
		data, err := json.Marshal(map[string]interface{}{
			"scope": []string{"logs.admin"},
		})
		Expect(err).ToNot(HaveOccurred())
		httpClient.resps = []response{{
			body:   data,
			status: http.StatusOK,
		}}

		c, err := client.Read("valid-token")
		Expect(err).ToNot(HaveOccurred())
		Expect(c.IsAdmin).To(BeTrue())
		Expect(c.Token).To(Equal("valid-token"))
	})

	It("does offline token validation through caching", func() {
		data, err := json.Marshal(map[string]interface{}{
			"scope": []string{"logs.admin"},
			"exp":   float64(time.Now().Add(time.Minute).UnixNano()) / 1e9,
		})
		Expect(err).ToNot(HaveOccurred())
		httpClient.resps = []response{{
			body:   data,
			status: http.StatusOK,
		}}

		client.Read("valid-token")
		client.Read("valid-token")

		Expect(len(httpClient.requests)).To(Equal(1))
	})

	It("does not cache an expired token", func() {
		data, err := json.Marshal(map[string]interface{}{
			"scope": []string{"logs.admin"},
			"exp":   float64(time.Now().Add(-time.Minute).UnixNano()) / 1e9,
		})

		Expect(err).ToNot(HaveOccurred())
		httpClient.resps = []response{{
			body:   data,
			status: http.StatusOK,
		}}

		client.Read("valid-token")
		client.Read("valid-token")

		Expect(len(httpClient.requests)).To(Equal(2))
	})

	It("calls UAA correctly", func() {
		token := "my-token"
		client.Read(token)

		r := httpClient.requests[0]

		Expect(r.Method).To(Equal(http.MethodPost))
		Expect(r.Header.Get("Content-Type")).To(
			Equal("application/x-www-form-urlencoded"),
		)
		Expect(r.URL.String()).To(Equal("https://uaa.com/check_token"))

		client, clientSecret, ok := r.BasicAuth()
		Expect(ok).To(BeTrue())
		Expect(client).To(Equal("some-client"))
		Expect(clientSecret).To(Equal("some-client-secret"))

		reqBytes, err := ioutil.ReadAll(r.Body)
		Expect(err).ToNot(HaveOccurred())
		urlValues, err := url.ParseQuery(string(reqBytes))
		Expect(err).ToNot(HaveOccurred())
		Expect(urlValues.Get("token")).To(Equal(token))
	})

	It("sets the last request latency metric", func() {
		client.Read("my-token")

		Expect(metrics.Get("cf_auth_proxy_last_uaa_latency")).ToNot(BeZero())
		Expect(metrics.GetUnit("cf_auth_proxy_last_uaa_latency")).To(Equal("nanoseconds"))
	})

	It("returns error when token is blank", func() {
		_, err := client.Read("")
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when UAA returns a non-200 response", func() {
		data, err := json.Marshal(map[string]interface{}{
			"error":             []string{"unauthorized"},
			"error_description": []string{"An Authentication object was not found in the SecurityContext"},
		})
		httpClient.resps = []response{{
			body:   data,
			status: http.StatusUnauthorized,
		}}

		c, err := client.Read("token")
		Expect(err).To(HaveOccurred())
		Expect(c.IsAdmin).To(BeFalse())
		Expect(c.Token).To(Equal(""))
	})

	It("returns false when scopes don't include doppler.firehose or logs.admin", func() {
		data, err := json.Marshal(map[string]interface{}{
			"scope": []string{"some-scope"},
		})
		Expect(err).ToNot(HaveOccurred())
		httpClient.resps = []response{{
			body:   data,
			status: http.StatusOK,
		}}

		c, err := client.Read("invalid-token")
		Expect(err).ToNot(HaveOccurred())

		Expect(c.IsAdmin).To(BeFalse())
	})

	It("returns false when the request fails", func() {
		httpClient.resps = []response{{
			err: errors.New("some-err"),
		}}

		_, err := client.Read("valid-token")
		Expect(err).To(HaveOccurred())
	})

	It("returns false when the response from the UAA is invalid", func() {
		httpClient.resps = []response{{
			body:   []byte("garbage"),
			status: http.StatusOK,
		}}

		_, err := client.Read("valid-token")
		Expect(err).To(HaveOccurred())
	})
})

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
