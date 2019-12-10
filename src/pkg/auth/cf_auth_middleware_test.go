package auth_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/auth"

	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testContext struct {
	spyOauth2ClientReader *spyOauth2ClientReader
	spyLogAuthorizer      *spyLogAuthorizer
	spyQueryParser        *spyQueryParser
	spyMetricRegistrar    *testing.SpyMetricRegistrar

	recorder *httptest.ResponseRecorder
	request  *http.Request
	provider auth.CFAuthMiddlewareProvider

	baseHandlerCalled  bool
	baseHandlerRequest *http.Request
	authHandler        http.Handler
}

const jsonError = `{
  "status": "error",
  "data": null,

  "errorType": "%s",
  "error": "%s",
  "warnings": []
}`

func setup(requestPath string) *testContext {
	spyOauth2ClientReader := newAdminChecker()
	spyLogAuthorizer := newSpyLogAuthorizer()
	spyQueryParser := newSpyQueryParser()
	spyMetricRegistrar := testing.NewSpyMetricRegistrar()

	provider := auth.NewCFAuthMiddlewareProvider(
		spyOauth2ClientReader,
		spyLogAuthorizer,
		spyQueryParser,
		spyMetricRegistrar,
		logger.NewTestLogger(),
	)

	request := httptest.NewRequest(http.MethodGet, requestPath, nil)
	request.Header.Set("Authorization", "bearer valid-token")

	tc := &testContext{
		spyOauth2ClientReader: spyOauth2ClientReader,
		spyLogAuthorizer:      spyLogAuthorizer,
		spyQueryParser:        spyQueryParser,
		spyMetricRegistrar:    spyMetricRegistrar,

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

			It("returns 401 if the user is not authorized for any of the query's source_ids", func() {
				tc := setup(`/api/v1/query?query=metric{source_id="some-id"}`)
				tc.spyQueryParser.sourceIds = []string{"some-id"}
				tc.spyLogAuthorizer.unauthorizedSourceIds = map[string]struct{}{
					"some-id": {},
				}

				tc.invokeAuthHandler()

				body, _ := ioutil.ReadAll(tc.recorder.Result().Body)
				Expect(tc.recorder.Code).To(Equal(http.StatusUnauthorized))

				errorBody := buildErrorJson(http.StatusUnauthorized, "there are no matching source IDs for your query")
				Expect(body).To(MatchJSON(errorBody))

				Expect(tc.baseHandlerCalled).To(BeFalse())
			})

			It("returns 422 if the query parser returns an error", func() {
				tc := setup(`/api/v1/query?query=metric`)
				tc.spyQueryParser.err = errors.New("query parser rejection")
				tc.invokeAuthHandler()

				body, _ := ioutil.ReadAll(tc.recorder.Result().Body)
				Expect(tc.recorder.Code).To(Equal(http.StatusUnprocessableEntity))

				errorBody := buildErrorJson(http.StatusUnprocessableEntity,
					"query parse error: query parser rejection")
				Expect(body).To(MatchJSON(errorBody))

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

			Expect(tc.spyMetricRegistrar.FetchHistogram(debug.AuthProxyRequestDurationSeconds)()).To(HaveLen(1))
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

			Expect(tc.spyMetricRegistrar.FetchHistogram(debug.AuthProxyRequestDurationSeconds)()).To(HaveLen(1))
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

	Describe("/api/v1/rules", func() {
		It("forwards the request to the handler if user is an admin", func() {
			tc := setup(`/api/v1/rules`)

			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusOK))
			Expect(tc.baseHandlerCalled).To(BeTrue())

			Expect(tc.spyOauth2ClientReader.token).To(Equal("bearer valid-token"))
		})

		It("404s if the user is not an admin", func() {
			tc := setup(`/api/v1/rules`)
			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
		})

		It("404s if the oauth2reader returns an error", func() {
			tc := setup(`/api/v1/rules`)
			tc.spyOauth2ClientReader.isAdminResult = true
			tc.spyOauth2ClientReader.err = errors.New("some-error")

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("/api/v1/alerts", func() {
		It("forwards the request to the handler if user is an admin", func() {
			tc := setup(`/api/v1/alerts`)

			tc.spyOauth2ClientReader.isAdminResult = true

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusOK))
			Expect(tc.baseHandlerCalled).To(BeTrue())

			Expect(tc.spyOauth2ClientReader.token).To(Equal("bearer valid-token"))
		})

		It("404s if the user is not an admin", func() {
			tc := setup(`/api/v1/alerts`)
			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
		})

		It("404s if the oauth2reader returns an error", func() {
			tc := setup(`/api/v1/alerts`)
			tc.spyOauth2ClientReader.isAdminResult = true
			tc.spyOauth2ClientReader.err = errors.New("some-error")

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("/health", func() {
		It("forwards the request to the handler health endpoint", func() {
			tc := setup("/health")

			tc.invokeAuthHandler()

			Expect(tc.recorder.Code).To(Equal(http.StatusOK))
			Expect(tc.baseHandlerCalled).To(BeTrue())

			Expect(tc.spyOauth2ClientReader.token).To(Equal(""))
		})
	})

	Describe("/rules", func() {
		Describe("/manager", func() {
			It("forwards the request to the rules endpoint handler", func() {
				tc := setup("/rules/manager")

				tc.invokeAuthHandler()

				Expect(tc.recorder.Code).ToNot(Equal(http.StatusNotFound))
				Expect(tc.baseHandlerCalled).To(BeTrue())
				Expect(tc.spyOauth2ClientReader.token).To(Equal(""))
			})
		})

		Describe("/manager/:id/group", func() {
			It("forwards the request to the rules group endpoint", func() {
				tc := setup("/rules/manager/id/group")

				tc.invokeAuthHandler()

				Expect(tc.recorder.Code).ToNot(Equal(http.StatusNotFound))
				Expect(tc.baseHandlerCalled).To(BeTrue())
				Expect(tc.spyOauth2ClientReader.token).To(Equal(""))
			})
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

func (s *spyLogAuthorizer) AvailableSourceIDs(token string) []string {
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

func buildErrorJson(statusCode int, message string) string {
	return fmt.Sprintf(jsonError, http.StatusText(statusCode), message)
}
