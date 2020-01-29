package api_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"

	. "github.com/cloudfoundry/metric-store-release/src/internal/api"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/pkg/tls"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	storagePathPrefix = "metric-store"
)

type ruleApiTestContext struct {
	ruleManager *testing.RuleManagerSpy
	addr        string
	httpClient  *http.Client
}

func (tc *ruleApiTestContext) Post(path string, payload []byte) (resp *http.Response, err error) {
	return tc.httpClient.Post(
		"https://"+tc.addr+path,
		"application/json",
		bytes.NewReader(payload),
	)
}

func (tc *ruleApiTestContext) Delete(path string) (resp *http.Response, err error) {
	req, err := http.NewRequest("DELETE", "https://"+tc.addr+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return tc.httpClient.Do(req)
}

var _ = Describe("Rules API", func() {
	var setup = func() (*ruleApiTestContext, func()) {
		spyRuleManager := testing.NewRuleManagerSpy()

		tlsConfig, err := sharedtls.NewMutualTLSConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).ToNot(HaveOccurred())

		insecureConnection, err := net.Listen("tcp", ":0")
		Expect(err).ToNot(HaveOccurred())

		secureConnection := tls.NewListener(insecureConnection, tlsConfig)
		mux := http.NewServeMux()

		rulesAPI := NewRulesAPI(spyRuleManager, logger.NewTestLogger(GinkgoWriter))
		rulesAPIRouter := rulesAPI.Router()
		mux.Handle("/rules/", http.StripPrefix("/rules", rulesAPIRouter))
		server := &http.Server{Handler: mux}
		go server.Serve(secureConnection)

		httpClient := &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		}

		return &ruleApiTestContext{
				ruleManager: spyRuleManager,
				httpClient:  httpClient,
				addr:        secureConnection.Addr().String(),
			}, func(server *http.Server) func() {
				return func() {
					server.Shutdown(context.Background())
				}
			}(server)
	}

	Describe("POST /rules/manager", func() {
		It("creates a rule manager", func() {
			tc, teardown := setup()
			defer teardown()

			payload := []byte(`
{
	"data": {
		"id": "app-metrics"
	}
}`)

			resp, err := tc.Post("/rules/manager", payload)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(201))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/json"))

			managerData := rulesclient.ManagerData{}
			json.NewDecoder(resp.Body).Decode(&managerData)

			Expect(managerData.Data.Id).To(Equal("app-metrics"))
			Expect(tc.ruleManager.ManagerIds()).To(ConsistOf("app-metrics"))
		})

		It("creates a rule manager with a generated id", func() {
			tc, teardown := setup()
			defer teardown()

			payload := []byte(`
{
	"data": {
	}
}`)

			resp, err := tc.Post("/rules/manager", payload)
			Expect(err).ToNot(HaveOccurred())

			managerData := rulesclient.ManagerData{}
			json.NewDecoder(resp.Body).Decode(&managerData)

			Expect(managerData.Data.Id).NotTo(BeEmpty())
			Expect(len(tc.ruleManager.ManagerIds())).To(Equal(1))
		})

		It("returns an error when invalid json is posted", func() {
			tc, teardown := setup()
			defer teardown()

			payload := []byte(`
{
	"data":
}`)

			resp, err := tc.Post("/rules/manager", payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(400))

			apiErrors := rulesclient.ApiErrors{}
			json.NewDecoder(resp.Body).Decode(&apiErrors)

			Expect(len(apiErrors.Errors)).To(Equal(1))
			Expect(apiErrors.Errors[0].Status).To(Equal(400))
			Expect(apiErrors.Errors[0].Title).To(ContainSubstring("invalid character"))
			Expect(len(tc.ruleManager.ManagerIds())).To(Equal(0))
		})

		It("returns an error when invalid manager is posted", func() {
			tc, teardown := setup()
			defer teardown()

			payload := []byte(`
{
	"data": {
		"alertmanager_url": "http://localhost:1234"
	}
}`)

			resp, err := tc.Post("/rules/manager", payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(400))

			apiErrors := rulesclient.ApiErrors{}
			json.NewDecoder(resp.Body).Decode(&apiErrors)

			Expect(len(apiErrors.Errors)).To(Equal(1))
			Expect(apiErrors.Errors[0].Status).To(Equal(400))
		})

		It("returns an error when the managerId has already been created", func() {
			tc, teardown := setup()
			defer teardown()

			tc.ruleManager.Create("app-metrics", "")
			Expect(len(tc.ruleManager.ManagerIds())).To(Equal(1))

			payload := []byte(`
{
	"data": {
		"id": "app-metrics"
	}
}`)

			resp, err := tc.Post("/rules/manager", payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(409))

			apiErrors := rulesclient.ApiErrors{}
			json.NewDecoder(resp.Body).Decode(&apiErrors)

			Expect(len(apiErrors.Errors)).To(Equal(1))
			Expect(apiErrors.Errors[0].Status).To(Equal(409))
			Expect(apiErrors.Errors[0].Title).To(ContainSubstring("already exists"))
			Expect(len(tc.ruleManager.ManagerIds())).To(Equal(1))
		})
	})

	Describe("POST /rules/manager/:manager_id/group", func() {
		It("creates a rule group", func() {
			tc, teardown := setup()
			defer teardown()

			tc.ruleManager.Create("app-metrics", "")
			Expect(len(tc.ruleManager.ManagerIds())).To(Equal(1))

			payload := []byte(`
{
	"data": {
		"name": "test-group",
		"rules": [
			{ "record": "sumCpuTotal", "expr": "sum(cpu)" }
		]
	}
}`)

			resp, err := tc.Post("/rules/manager/app-metrics/group", payload)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(201))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/json"))

			ruleGroupData := rulesclient.RuleGroupData{}
			err = json.NewDecoder(resp.Body).Decode(&ruleGroupData)
			Expect(err).NotTo(HaveOccurred())

			Expect(ruleGroupData.Data.Name).To(Equal("test-group"))
			Expect(len(tc.ruleManager.RuleGroups())).To(Equal(1))
			Expect(len(tc.ruleManager.RuleGroupForManager("app-metrics").Rules)).To(Equal(1))
		})

		It("returns an error when invalid json is posted", func() {
			tc, teardown := setup()
			defer teardown()

			tc.ruleManager.Create("app-metrics", "")
			Expect(len(tc.ruleManager.ManagerIds())).To(Equal(1))

			payload := []byte(`
{
	"data":
}`)

			resp, err := tc.Post("/rules/manager/app-metrics/group", payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(400))

			apiErrors := rulesclient.ApiErrors{}
			json.NewDecoder(resp.Body).Decode(&apiErrors)

			Expect(len(apiErrors.Errors)).To(Equal(1))
			Expect(apiErrors.Errors[0].Status).To(Equal(400))
			Expect(apiErrors.Errors[0].Title).To(ContainSubstring("invalid character"))
		})

		It("returns an error when invalid rule group is posted", func() {
			tc, teardown := setup()
			defer teardown()

			tc.ruleManager.Create("app-metrics", "")
			Expect(len(tc.ruleManager.ManagerIds())).To(Equal(1))

			payload := []byte(`
{
	"data": {
		"rules": [
			{ "record": "sumCpuTotal", "expr": "sum(cpu)" }
		]
	}
}`)

			resp, err := tc.Post("/rules/manager/app-metrics/group", payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(400))

			apiErrors := rulesclient.ApiErrors{}
			json.NewDecoder(resp.Body).Decode(&apiErrors)

			Expect(len(apiErrors.Errors)).To(Equal(1))
			Expect(apiErrors.Errors[0].Status).To(Equal(400))
		})

		It("returns an error when the managerId has not been created", func() {
			tc, teardown := setup()
			defer teardown()

			payload := []byte(`
{
	"data": {
		"name": "test-group",
		"rules": [
			{ "record": "sumCpuTotal", "expr": "sum(cpu)" }
		]
	}
}`)

			resp, err := tc.Post("/rules/manager/app-metrics/group", payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(400))

			apiErrors := rulesclient.ApiErrors{}
			json.NewDecoder(resp.Body).Decode(&apiErrors)

			Expect(len(apiErrors.Errors)).To(Equal(1))
			Expect(apiErrors.Errors[0].Status).To(Equal(400))
			Expect(apiErrors.Errors[0].Title).To(ContainSubstring("does not exist"))
		})
	})

	Describe("DELETE /rules/manager/:manager_id", func() {
		It("deletes a rule manager", func() {
			tc, teardown := setup()
			defer teardown()

			tc.ruleManager.Create("app-metrics", "")

			resp, err := tc.Delete("/rules/manager/app-metrics")
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(204))
			Expect(tc.ruleManager.ManagerIds()).NotTo(ContainElement("app-metrics"))
		})

		It("returns an error when the managerId does not exist", func() {
			tc, teardown := setup()
			defer teardown()

			Expect(tc.ruleManager.ManagerIds()).To(BeEmpty())

			resp, err := tc.Delete("/rules/manager/app-metrics")
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(404))

			apiErrors := rulesclient.ApiErrors{}
			json.NewDecoder(resp.Body).Decode(&apiErrors)

			Expect(len(apiErrors.Errors)).To(Equal(1))
			Expect(apiErrors.Errors[0].Status).To(Equal(404))
			Expect(apiErrors.Errors[0].Title).To(ContainSubstring("does not exist"))
		})

		It("doesn't do anything and returns an error if the HTTP verb is not DELETE", func() {
			tc, teardown := setup()
			defer teardown()

			resp, err := tc.Post("/rules/manager/app-metrics", nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(405))
		})
	})
})
