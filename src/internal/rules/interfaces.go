package rules

import (
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	"github.com/prometheus/prometheus/rules"
)

const (
	ManagerExistsError    = ManagerStoreError("The manager already exists")
	ManagerNotExistsError = ManagerStoreError("The manager does not exist")
)

type RuleManager interface {
	CreateManager(managerId, alertmanagerAddr string) error
	DeleteManager(managerId string) error
	UpsertRuleGroup(managerId string, ruleGroup *rulesclient.RuleGroup) error
	RuleGroups() []*rules.Group
	AlertingRules() []*rules.AlertingRule
	Alertmanagers() []*url.URL
	DroppedAlertmanagers() []*url.URL
}

type RuleManagers interface {
	Create(managerId string, managerFile string, alertmanagerAddr string) error
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

	// prometheus api v1 rulesRetriever
	RuleGroups() []*rules.Group
	AlertingRules() []*rules.AlertingRule

	// prometheus api v1 alertmanagerRetriever
	Alertmanagers() []*url.URL
	DroppedAlertmanagers() []*url.URL
}
