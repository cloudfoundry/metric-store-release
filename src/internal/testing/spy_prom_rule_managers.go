package testing

import (
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/internal/rules"
	prom_rules "github.com/prometheus/prometheus/rules"
)

type PromRuleManagersSpy struct {
	managerIds map[string]struct{}
}

func NewPromRuleManagersSpy() *PromRuleManagersSpy {
	return &PromRuleManagersSpy{
		managerIds: make(map[string]struct{}, 0),
	}
}

func (p *PromRuleManagersSpy) Create(managerId string, _ string, _ string) error {
	p.managerIds[managerId] = struct{}{}
	return nil
}

func (p *PromRuleManagersSpy) Delete(managerId string) error {
	_, exists := p.managerIds[managerId]

	if !exists {
		return rules.ManagerNotExistsError
	}

	delete(p.managerIds, managerId)

	return nil
}

func (p *PromRuleManagersSpy) Reload() error {
	panic("not implemented") // TODO: Implement
}

func (p *PromRuleManagersSpy) RuleGroups() []*prom_rules.Group {
	panic("not implemented") // TODO: Implement
}

func (p *PromRuleManagersSpy) AlertingRules() []*prom_rules.AlertingRule {
	panic("not implemented") // TODO: Implement
}

func (p *PromRuleManagersSpy) Alertmanagers() []*url.URL {
	panic("not implemented") // TODO: Implement
}

func (p *PromRuleManagersSpy) DroppedAlertmanagers() []*url.URL {
	panic("not implemented") // TODO: Implement
}

func (p *PromRuleManagersSpy) DeleteAll() error {
	panic("not implemented") // TODO: Implement
}

func (p *PromRuleManagersSpy) ManagerIds() []string {
	var managerIds []string

	for managerId := range p.managerIds {
		managerIds = append(managerIds, managerId)
	}

	return managerIds
}
