package rulesclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	prom_config "github.com/prometheus/prometheus/config"
)

var CertificateRegexp = regexp.MustCompile(`^-----BEGIN .+?-----([\s\S]*)-----END .+?-----\s?$`)

type ApiErrors struct {
	Errors []ApiError `json:"errors"`
}

type ApiError struct {
	Status int    `json:"status"`
	Title  string `json:"title"`
}

func (a *ApiError) Error() string {
	return a.Title
}

type Manager interface {
	Id() string
	AlertManagers() *prom_config.AlertmanagerConfigs
}

type RulesClient struct {
	httpClient *http.Client
	addr       string
	private    bool
	log        *logger.Logger
}

func NewRulesClient(addr string, tlsConfig *tls.Config, opts ...RulesClientOption) *RulesClient {
	client := &RulesClient{
		addr:    addr,
		private: false,
		log:     logger.NewNop(),
	}

	client.httpClient = &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}

	for _, o := range opts {
		o(client)
	}

	return client
}

type RulesClientOption func(*RulesClient)

func WithRulesClientLogger(log *logger.Logger) RulesClientOption {
	return func(client *RulesClient) {
		client.log = log
	}
}

func WithRulesClientPrivate() RulesClientOption {
	return func(client *RulesClient) {
		client.private = true
	}
}

func WithRulesClientTimeout(timeout time.Duration) RulesClientOption {
	return func(client *RulesClient) {
		client.httpClient.Timeout = timeout
	}
}

func (c *RulesClient) path() string {
	path := "https://" + c.addr

	if c.private {
		path += "/private"
	}

	return path
}

func (c *RulesClient) post(path string, payload []byte) (resp *http.Response, err error) {
	return c.httpClient.Post(
		c.path()+path,
		"application/json",
		bytes.NewReader(payload),
	)
}

func (c *RulesClient) destroy(path string) (resp *http.Response, err error) {
	req, err := http.NewRequest("DELETE", c.path()+path, bytes.NewReader([]byte("")))
	if err != nil {
		return nil, err
	}

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return resp, err
	}

	return resp, err
}

func (c *RulesClient) CreateManager(managerId string, alertmanagerConfigs *prom_config.AlertmanagerConfigs) (Manager, *ApiError) {
	managerConfig := NewManagerConfig(managerId, alertmanagerConfigs)
	payload, err := managerConfig.ToJSON()
	if err != nil {
		return nil, apiError(http.StatusBadRequest, err)
	}

	resp, err := c.post("/rules/manager", payload)
	if err != nil {
		return nil, apiError(http.StatusInternalServerError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, extractFirstError(resp)
	}

	modifiedManagerConfig, err := ManagerConfigFromJSON(resp.Body)
	if err != nil {
		return nil, apiError(http.StatusInternalServerError, err)
	}
	return modifiedManagerConfig, nil
}

func (c *RulesClient) UpsertRuleGroup(managerId string, ruleGroup RuleGroup) (*RuleGroup, *ApiError) {
	data := RuleGroupData{
		Data: ruleGroup,
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, apiError(http.StatusBadRequest, err)
	}

	resp, err := c.post("/rules/manager/"+managerId+"/group", payload)
	if err != nil {
		return nil, apiError(http.StatusInternalServerError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, extractFirstError(resp)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, apiError(http.StatusInternalServerError, err)
	}

	responseBody := RuleGroupData{}
	json.Unmarshal(body, &responseBody)
	err = json.Unmarshal(body, &responseBody)
	if err != nil {
		return nil, apiError(http.StatusInternalServerError, err)
	}

	return &responseBody.Data, nil
}

func (c *RulesClient) DeleteManager(managerId string) *ApiError {
	resp, err := c.destroy("/rules/manager/" + managerId)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return extractFirstError(resp)
	}

	return nil
}

func extractFirstError(resp *http.Response) *ApiError {
	var apiErrors *ApiErrors
	title := ""
	status := 0

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return apiError(http.StatusBadRequest, err)
	}

	err = json.Unmarshal(body, &apiErrors)
	if err != nil {
		return apiError(http.StatusBadRequest, err)
	}

	if len(apiErrors.Errors) > 0 {
		title = apiErrors.Errors[0].Title
		status = apiErrors.Errors[0].Status
	}
	return &ApiError{Status: status, Title: title}
}

func apiError(status int, err error) *ApiError {
	return &ApiError{Status: status, Title: err.Error()}
}
