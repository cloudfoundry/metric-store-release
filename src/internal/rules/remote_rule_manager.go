package rules

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	prom_api_client "github.com/prometheus/client_golang/api"
	prom_versioned_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/rules"
)

type RemoteRuleManager struct {
	rulesClient *rulesclient.RulesClient
	apiClient   prom_versioned_api_client.API
}

func NewRemoteRuleManager(addr string, tlsConfig *tls.Config) *RemoteRuleManager {
	rulesClient := rulesclient.NewRulesClient(
		addr,
		tlsConfig,
		rulesclient.WithRulesClientPrivate(),
	)

	promAPIClient, _ := prom_api_client.NewClient(prom_api_client.Config{
		Address: fmt.Sprintf("https://%s/private", addr),
		RoundTripper: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	})
	apiClient := prom_versioned_api_client.NewAPI(promAPIClient)

	return &RemoteRuleManager{
		rulesClient: rulesClient,
		apiClient:   apiClient,
	}
}

func (r *RemoteRuleManager) CreateManager(managerId string, alertmanagerConfigs *prom_config.AlertmanagerConfigs) error {
	_, err := r.rulesClient.CreateManager(managerId, alertmanagerConfigs)

	if err != nil {
		switch err.Status {
		case http.StatusConflict:
			return ManagerExistsError
		default:
			return err
		}
	}

	return nil
}

func (r *RemoteRuleManager) DeleteManager(managerId string) error {
	err := r.rulesClient.DeleteManager(managerId)

	if err != nil {
		switch err.Status {
		case http.StatusNotFound:
			return ManagerNotExistsError
		default:
			return err
		}
	}

	return nil
}

func (r *RemoteRuleManager) UpsertRuleGroup(managerId string, ruleGroup *rulesclient.RuleGroup) error {
	_, err := r.rulesClient.UpsertRuleGroup(managerId, *ruleGroup)

	if err != nil {
		switch err.Status {
		case http.StatusNotFound:
			return ManagerNotExistsError
		default:
			return err
		}
	}

	return nil
}

func (r *RemoteRuleManager) RuleGroups() []*rules.Group {
	var ruleGroups []*rules.Group
	return ruleGroups
}

func (r *RemoteRuleManager) AlertingRules() []*rules.AlertingRule {
	var alertingRules []*rules.AlertingRule
	return alertingRules
}

func (r *RemoteRuleManager) Alertmanagers() []*url.URL {
	var alertmanagers []*url.URL

	resp, err := r.apiClient.AlertManagers(context.Background())
	if err != nil {
		return alertmanagers
	}

	for _, alertmanager := range resp.Active {
		url, err := url.Parse(alertmanager.URL)
		if err != nil {
			return alertmanagers
		}

		alertmanagers = append(alertmanagers, url)
	}

	return alertmanagers
}

func (r *RemoteRuleManager) DroppedAlertmanagers() []*url.URL {
	var alertmanagers []*url.URL

	resp, err := r.apiClient.AlertManagers(context.Background())
	if err != nil {
		return alertmanagers
	}

	for _, alertmanager := range resp.Dropped {
		url, err := url.Parse(alertmanager.URL)
		if err != nil {
			return alertmanagers
		}

		alertmanagers = append(alertmanagers, url)
	}

	return alertmanagers
}
