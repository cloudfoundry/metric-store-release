package rules

import (
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/rules"
)

type LocalRuleManager struct {
	rulesManagerFiles *RuleManagerFiles
	promRuleManagers  RuleManagers
}

func NewLocalRuleManager(storagePath string, promRuleManagers RuleManagers) *LocalRuleManager {
	return &LocalRuleManager{
		rulesManagerFiles: NewRuleManagerFiles(storagePath),
		promRuleManagers:  promRuleManagers,
	}
}

func (l *LocalRuleManager) CreateManager(managerId string, alertManagers *prom_config.AlertmanagerConfigs) (*Manager, error) {
	rulesFilePath, err := l.rulesManagerFiles.Create(managerId, alertManagers)
	if err != nil {
		return nil, err
	}

	err = l.promRuleManagers.Create(managerId, rulesFilePath, alertManagers)
	if err != nil {
		return nil, err
	}

	return NewManager(managerId, alertManagers), nil
}

func (l *LocalRuleManager) DeleteManager(managerId string) error {
	err := l.rulesManagerFiles.Delete(managerId)
	if err != nil {
		return err
	}

	return l.promRuleManagers.Delete(managerId)
}

func (l *LocalRuleManager) UpsertRuleGroup(managerId string, ruleGroup *rulesclient.RuleGroup) error {
	promRuleGroup, err := ruleGroup.ConvertToPromRuleGroup()
	if err != nil {
		return err
	}

	err = l.rulesManagerFiles.UpsertRuleGroup(managerId, promRuleGroup)
	if err != nil {
		return err
	}

	return l.promRuleManagers.Reload()
}

func (l *LocalRuleManager) RuleGroups() []*rules.Group {
	return l.promRuleManagers.RuleGroups()
}

func (l *LocalRuleManager) AlertingRules() []*rules.AlertingRule {
	return l.promRuleManagers.AlertingRules()
}

func (l *LocalRuleManager) Alertmanagers() []*url.URL {
	return l.promRuleManagers.Alertmanagers()
}

func (l *LocalRuleManager) DroppedAlertmanagers() []*url.URL {
	return l.promRuleManagers.DroppedAlertmanagers()
}
