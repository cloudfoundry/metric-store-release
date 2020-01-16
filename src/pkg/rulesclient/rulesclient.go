package rulesclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
)

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

type ErrorNotCreated struct {
	Title string
}

func (e *ErrorNotCreated) Error() string {
	panic("not implemented")
}

func (c *RulesClient) CreateManager(managerId, alertmanagerAddr string) (*Manager, error) {
	manager := Manager{
		Id:              managerId,
		AlertManagerUrl: alertmanagerAddr,
	}
	data := ManagerData{
		Data: manager,
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	resp, err := c.post("/rules/manager", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.createError(resp)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	responseBody := ManagerData{}
	err = json.Unmarshal(body, &responseBody)
	if err != nil {
		return nil, err
	}

	return &responseBody.Data, nil
}

func (c *RulesClient) UpsertRuleGroup(managerId string, ruleGroup RuleGroup) (*RuleGroup, error) {
	data := RuleGroupData{
		Data: ruleGroup,
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	resp, err := c.post("/rules/manager/"+managerId+"/group", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.createError(resp)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	responseBody := RuleGroupData{}
	json.Unmarshal(body, &responseBody)
	err = json.Unmarshal(body, &responseBody)
	if err != nil {
		return nil, err
	}

	return &responseBody.Data, nil
}

func (c *RulesClient) createError(resp *http.Response) error {
	var apiErrors *ApiErrors

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, &apiErrors)
	if err != nil {
		return err
	}

	title := ""
	if len(apiErrors.Errors) > 0 {
		title = apiErrors.Errors[0].Title
	}
	return &ErrorNotCreated{Title: title}
}
