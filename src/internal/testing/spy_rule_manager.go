package testing

import (
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	prom_rules "github.com/prometheus/prometheus/rules"
)

type RuleManagerSpy struct {
	rules                map[string]*rulesclient.RuleGroup
	alertmanagers        []*url.URL
	droppedAlertmanagers []*url.URL
}

func NewRuleManagerSpy() *RuleManagerSpy {
	return &RuleManagerSpy{
		rules: make(map[string]*rulesclient.RuleGroup),
	}
}

func (r *RuleManagerSpy) Create(managerId, alertmanagerAddr string) error {
	if _, exists := r.rules[managerId]; exists {
		return rules.ManagerExistsError
	}

	r.rules[managerId] = nil

	if alertmanagerAddr != "" {
		url, err := url.Parse(alertmanagerAddr)
		if err != nil {
			return err
		}
		r.alertmanagers = append(r.alertmanagers, url)
	}

	return nil
}

func (r *RuleManagerSpy) ManagerIds() []string {
	managerIds := []string{}

	for managerId, _ := range r.rules {
		managerIds = append(managerIds, managerId)
	}

	return managerIds
}

func (r *RuleManagerSpy) UpsertRuleGroup(managerId string, ruleGroup *rulesclient.RuleGroup) error {
	if _, exists := r.rules[managerId]; !exists {
		return rules.ManagerNotExistsError
	}

	r.rules[managerId] = ruleGroup

	return nil
}

func (r *RuleManagerSpy) RuleGroups() []*prom_rules.Group {
	ruleGroups := []*prom_rules.Group{}

	for _, ruleGroup := range r.rules {
		if ruleGroup == nil {
			continue
		}

		ruleGroups = append(ruleGroups, &prom_rules.Group{})
	}

	return ruleGroups
}

func (r *RuleManagerSpy) RuleGroupForManager(managerId string) *rulesclient.RuleGroup {
	return r.rules[managerId]
}

func (r *RuleManagerSpy) AlertingRules() []*prom_rules.AlertingRule {
	panic("not implemented")
}

func (r *RuleManagerSpy) Alertmanagers() []*url.URL {
	return r.alertmanagers
}

func (r *RuleManagerSpy) AddDroppedAlertmanagers(urls []*url.URL) {
	r.droppedAlertmanagers = urls
}

func (r *RuleManagerSpy) DroppedAlertmanagers() []*url.URL {
	return r.droppedAlertmanagers
}
