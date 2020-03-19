package rules

import (
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/rules"
)

const (
	ManagerExistsError    = ManagerStoreError("The manager already exists")
	ManagerNotExistsError = ManagerStoreError("The manager does not exist")
)

type RuleManager interface {
	CreateManager(managerId string, alertmanagerConfigs *prom_config.AlertmanagerConfigs) error
	DeleteManager(managerId string) error
	UpsertRuleGroup(managerId string, ruleGroup *rulesclient.RuleGroup) error
	RuleGroups() []*rules.Group
	AlertingRules() []*rules.AlertingRule
	Alertmanagers() []*url.URL
	DroppedAlertmanagers() []*url.URL
}

type RuleManagers interface {
	Create(managerId string, managerFile string, alertManagers *prom_config.AlertmanagerConfigs) error
	Delete(managerId string) error
	DeleteAll() error
	Reload() error
	RuleGroups() []*rules.Group
	AlertingRules() []*rules.AlertingRule
	Alertmanagers() []*url.URL
	DroppedAlertmanagers() []*url.URL
}

type PromManagerWrapper interface {
	Start() error
	Reload() error

	// implements prometheus api v1 rulesRetriever interface
	RuleGroups() []*rules.Group
	AlertingRules() []*rules.AlertingRule

	// implements prometheus api v1 alertmanagerRetriever interface
	Alertmanagers() []*url.URL
	DroppedAlertmanagers() []*url.URL
}
