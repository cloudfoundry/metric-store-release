package auth_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry/metric-store/src/pkg/auth"

	"github.com/cloudfoundry/metric-store/src/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testContext struct {
	spyOauth2ClientReader *spyOauth2ClientReader
	spyLogAuthorizer      *spyLogAuthorizer
	spyQueryParser        *spyQueryParser
	spyMetrics            *testing.SpyMetrics

	recorder *httptest.ResponseRecorder
	request  *http.Request
	provider auth.CFAuthMiddlewareProvider

	baseHandlerCalled  bool
	baseHandlerRequest *http.Request
	authHandler        http.Handler
}

func setup(requestPath string) *testContext {
	spyOauth2ClientReader := newAdminChecker()
	spyLogAuthorizer := newSpyLogAuthorizer()
	spyQueryParser := newSpyQueryParser()
	spyMetrics := testing.NewSpyMetrics()

	provider := auth.NewCFAuthMiddlewareProvider(
		spyOauth2ClientReader,
		spyLogAuthorizer,
		spyQueryParser,
		spyMetrics,
	)

	request := httptest.NewRequest(http.MethodGet, requestPath, nil)
	request.Header.Set("Authorization", "bearer valid-token")

	tc := &testContext{
		spyOauth2ClientReader: spyOauth2ClientReader,
		spyLogAuthorizer:      spyLogAuthorizer,
		spyQueryParser:        spyQueryParser,
		spyMetrics:            spyMetrics,

		recorder: httptest.NewRecorder(),
		request:  request,
		provider: provider,
	}

	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc.baseHandlerCalled = true
		tc.baseHandlerRequest = r
	})
	tc.authHandler = provider.Middleware(baseHandler)

	return tc
}

func (tc *testContext) invokeAuthHandler() {
	tc.authHandler.ServeHTTP(tc.recorder, tc.request)
}

var _ = Describe("CfAuthMiddleware", func() {
	Describe("/api/v1/read", func() {
		It("forwards the request to the handler if user is an admin", func() {
			tc := setup("/api/v1/read/metric")

			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusOK))
			Expect(tc.baseHandlerCalled).To(BeTrue())

			Expect(tc.spyOauth2ClientReader.token).To(Equal("bearer valid-token"))
		})

		It("returns 404 Not Found if there's no authorization header present", func() {
			tc := setup("/api/v1/read/memory")
			tc.request.Header.Del("Authorization")

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
			Expect(tc.baseHandlerCalled).To(BeFalse())
		})

		It("returns 404 Not Found if Oauth2ClientReader returns an error", func() {
			tc := setup("/")
			tc.spyOauth2ClientReader.err = errors.New("some-error")

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
			Expect(tc.baseHandlerCalled).To(BeFalse())
		})

		It("temporarily returns 404 Not Found if user is not an admin", func() {
			tc := setup("/api/v1/read/memory")
			tc.spyOauth2ClientReader.isAdminResult = false

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
			Expect(tc.baseHandlerCalled).To(BeFalse())
		})

		It("records the total time of the query as a metric", func() {
			tc := setup("/api/v1/read/memory")
			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.spyMetrics.Get("cf_auth_proxy_total_read_time")).ToNot(Equal(testing.UNDEFINED_METRIC))
			Expect(tc.spyMetrics.Get("cf_auth_proxy_total_read_time")).ToNot(BeZero())
		})
	})

	Describe("/api/v1/query", func() {
		It("forwards the request to the handler if user is an admin", func() {
			tc := setup(`/api/v1/query?query=metric`)
			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusOK))
			Expect(tc.baseHandlerCalled).To(BeTrue())

			Expect(tc.spyOauth2ClientReader.token).To(Equal("bearer valid-token"))
			Expect(tc.baseHandlerRequest.URL.Query().Get("query")).To(Equal("metric"))
		})

		Context("if the user is not a logs admin", func() {
			It("forwards the request to the handler if non-admin user has log access", func() {
				tc := setup(`/api/v1/query?query=metric{source_id="some-id"}`)
				tc.spyQueryParser.sourceIds = []string{"some-id"}

				tc.invokeAuthHandler()

				Expect(tc.recorder.Code).To(Equal(http.StatusOK))
				Expect(tc.baseHandlerCalled).To(BeTrue())
			})

			It("returns 422 if the user is not authorized for any of the query's source_ids", func() {
				tc := setup(`/api/v1/query?query=metric{source_id="some-id"}`)
				tc.spyQueryParser.sourceIds = []string{"some-id"}
				tc.spyLogAuthorizer.unauthorizedSourceIds = map[string]struct{}{
					"some-id": {},
				}

				tc.invokeAuthHandler()

				body, _ := ioutil.ReadAll(tc.recorder.Result().Body)
				Expect(tc.recorder.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(string(body)).To(ContainSubstring("query parse error"))
				Expect(tc.baseHandlerCalled).To(BeFalse())
			})

			It("returns 422 if the query parser returns an error", func() {
				tc := setup(`/api/v1/query?query=metric`)
				tc.spyQueryParser.err = errors.New("query parser rejection")
				tc.invokeAuthHandler()

				body, _ := ioutil.ReadAll(tc.recorder.Result().Body)
				Expect(tc.recorder.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(string(body)).To(ContainSubstring("query parse error"))
				Expect(tc.baseHandlerCalled).To(BeFalse())
			})
		})

		It("returns 404 Not Found if Oauth2ClientReader returns an error", func() {
			tc := setup(`/api/v1/query?query=metric`)
			tc.spyOauth2ClientReader.err = errors.New("some-error")

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
			Expect(tc.baseHandlerCalled).To(BeFalse())
		})

		It("records the total time of the query as a metric", func() {
			tc := setup(`/api/v1/query?query=metric`)
			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.spyMetrics.Get("cf_auth_proxy_total_query_time")).ToNot(Equal(testing.UNDEFINED_METRIC))
			Expect(tc.spyMetrics.Get("cf_auth_proxy_total_query_time")).ToNot(BeZero())
		})
	})

	Describe("/api/v1/query_range", func() {
		It("forwards the /api/v1/query_range request to the handler if user is an admin", func() {
			tc := setup(`/api/v1/query_range?query=metric`)
			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusOK))
			Expect(tc.baseHandlerCalled).To(BeTrue())

			Expect(tc.spyOauth2ClientReader.token).To(Equal("bearer valid-token"))
			Expect(tc.baseHandlerRequest.URL.Query().Get("query")).To(Equal("metric"))
		})

		It("records the total time of the query as a metric", func() {
			tc := setup(`/api/v1/query_range?query=metric`)
			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.spyMetrics.Get("cf_auth_proxy_total_query_time")).ToNot(Equal(testing.UNDEFINED_METRIC))
			Expect(tc.spyMetrics.Get("cf_auth_proxy_total_query_time")).ToNot(BeZero())
		})
	})

	Describe("/api/v1/labels", func() {
		It("forwards the request to the handler if user is an admin", func() {
			tc := setup(`/api/v1/labels`)

			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusOK))
			Expect(tc.baseHandlerCalled).To(BeTrue())

			Expect(tc.spyOauth2ClientReader.token).To(Equal("bearer valid-token"))
		})

		It("404s if the user is not an admin", func() {
			tc := setup(`/api/v1/labels`)
			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
		})

		It("404s if the oauth2reader returns an error", func() {
			tc := setup(`/api/v1/labels`)
			tc.spyOauth2ClientReader.isAdminResult = true
			tc.spyOauth2ClientReader.err = errors.New("some-error")

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("/api/v1/series", func() {
		It("forwards the request to the handler if user is an admin", func() {
			tc := setup(`/api/v1/series`)

			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusOK))
			Expect(tc.baseHandlerCalled).To(BeTrue())

			Expect(tc.spyOauth2ClientReader.token).To(Equal("bearer valid-token"))
		})

		It("404s if the user is not an admin", func() {
			tc := setup(`/api/v1/series`)
			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
		})

		It("404s if the oauth2reader returns an error", func() {
			tc := setup(`/api/v1/series`)
			tc.spyOauth2ClientReader.isAdminResult = true
			tc.spyOauth2ClientReader.err = errors.New("some-error")

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("/api/v1/label/<metric-name>/values", func() {
		It("forwards the request to the handler if user is an admin", func() {
			tc := setup(`/api/v1/label/metric-name/values`)

			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusOK))
			Expect(tc.baseHandlerCalled).To(BeTrue())

			Expect(tc.spyOauth2ClientReader.token).To(Equal("bearer valid-token"))
		})
		It("does not forward if invalid metric name", func() {
			tc := setup(`/api/v1/label/metric/name/values`)

			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
		})

		It("404s if the user is not an admin", func() {
			tc := setup(`/api/v1/label/metric-name/values`)
			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
		})

		It("404s if the oauth2reader returns an error", func() {
			tc := setup(`/api/v1/label/metric-name/values`)
			tc.spyOauth2ClientReader.isAdminResult = true
			tc.spyOauth2ClientReader.err = errors.New("some-error")

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
		})
	})
})

type spyOauth2ClientReader struct {
	token         string
	isAdminResult bool
	client        string
	user          string
	err           error
}

func newAdminChecker() *spyOauth2ClientReader {
	return &spyOauth2ClientReader{}
}

func (s *spyOauth2ClientReader) Read(token string) (auth.Oauth2ClientContext, error) {
	s.token = token
	return auth.Oauth2ClientContext{
		IsAdmin: s.isAdminResult,
		Token:   token,
	}, s.err
}

type spyLogAuthorizer struct {
	unauthorizedSourceIds map[string]struct{}
	sourceIdsCalledWith   map[string]struct{}
	token                 string
	available             []string
	availableCalled       int
}

func newSpyLogAuthorizer() *spyLogAuthorizer {
	return &spyLogAuthorizer{
		unauthorizedSourceIds: make(map[string]struct{}),
		sourceIdsCalledWith:   make(map[string]struct{}),
	}
}

func (s *spyLogAuthorizer) IsAuthorized(sourceId string, clientToken string) bool {
	s.sourceIdsCalledWith[sourceId] = struct{}{}
	s.token = clientToken

	_, exists := s.unauthorizedSourceIds[sourceId]

	return !exists
}

func (s *spyLogAuthorizer) AvailableSourceIds(token string) []string {
	s.availableCalled++
	s.token = token
	return s.available
}

type spyQueryParser struct {
	sourceIds []string
	err       error
}

func newSpyQueryParser() *spyQueryParser {
	return &spyQueryParser{}
}

func (s *spyQueryParser) ExtractSourceIds(query string) ([]string, error) {
	return s.sourceIds, s.err
}
