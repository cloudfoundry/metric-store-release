package rules

import (
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	"github.com/prometheus/prometheus/rules"
)

type RuleManagers interface {
	Create(string, string, string) error
	Reload() error
	RuleGroups() []*rules.Group
	AlertingRules() []*rules.AlertingRule
	Alertmanagers() []*url.URL
	DroppedAlertmanagers() []*url.URL
}

type LocalRuleManager struct {
	rulesManagerFile *RuleManagerFile
	promRuleManagers RuleManagers
}

func NewLocalRuleManager(storagePath string, promRuleManagers RuleManagers) *LocalRuleManager {
	return &LocalRuleManager{
		rulesManagerFile: NewRuleManagerFile(storagePath),
		promRuleManagers: promRuleManagers,
	}
}

func (l *LocalRuleManager) Create(managerId, alertmanagerAddr string) error {
	managerFile, err := l.rulesManagerFile.Create(managerId, alertmanagerAddr)
	if err != nil {
		return err
	}

	err = l.promRuleManagers.Create(managerId, managerFile, alertmanagerAddr)
	if err != nil {
		return err
	}

	return nil
}

func (l *LocalRuleManager) UpsertRuleGroup(managerId string, ruleGroup *rulesclient.RuleGroup) error {
	promRuleGroup, err := ruleGroup.ConvertToPromRuleGroup()
	if err != nil {
		return err
	}

	err = l.rulesManagerFile.UpsertRuleGroup(managerId, promRuleGroup)
	if err != nil {
		return err
	}

	err = l.promRuleManagers.Reload()
	if err != nil {
		return err
	}

	return nil
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
