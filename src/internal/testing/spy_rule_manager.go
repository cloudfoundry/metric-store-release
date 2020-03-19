package testing

import (
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	prom_config "github.com/prometheus/prometheus/config"
	prom_rules "github.com/prometheus/prometheus/rules"
)

type RuleManagerSpy struct {
	rules                map[string]*rulesclient.RuleGroup
	alertmanagers        []*url.URL
	droppedAlertmanagers []*url.URL
	methodsCalled        chan string
}

func NewRuleManagerSpy() *RuleManagerSpy {
	return &RuleManagerSpy{
		rules:         make(map[string]*rulesclient.RuleGroup),
		methodsCalled: make(chan string, 10),
	}
}

func (r *RuleManagerSpy) CreateManager(managerId string, alertManagers *prom_config.AlertmanagerConfigs) error {
	r.methodsCalled <- "CreateManager"

	if _, exists := r.rules[managerId]; exists {
		return rules.ManagerExistsError
	}

	r.rules[managerId] = nil

	if alertManagers != nil {
		ams := alertManagers.ToMap()
		for _, am := range ams {
			for _, sc := range am.ServiceDiscoveryConfig.StaticConfigs {
				for _, t := range sc.Targets {
					addr := string(t["__address__"])
					if scheme, ok := t["__scheme__"]; ok {
						addr = string(scheme) + "://" + addr
					}

					u, err := url.Parse(addr)
					if err != nil {
						return err
					}
					r.alertmanagers = append(r.alertmanagers, u)
				}
			}
		}
	}

	return nil
}

func (r *RuleManagerSpy) DeleteManager(managerId string) error {
	r.methodsCalled <- "DeleteManager"

	if _, exists := r.rules[managerId]; !exists {
		return rules.ManagerNotExistsError
	}

	delete(r.rules, managerId)

	return nil
}

func (r *RuleManagerSpy) ManagerIds() []string {
	managerIds := []string{}

	for managerId := range r.rules {
		managerIds = append(managerIds, managerId)
	}

	return managerIds
}

func (r *RuleManagerSpy) MethodsCalled() []string {
	var methods []string

	for {
		select {
		case method := <-r.methodsCalled:
			methods = append(methods, method)
		default:
			return methods
		}
	}
}

func (r *RuleManagerSpy) UpsertRuleGroup(managerId string, ruleGroup *rulesclient.RuleGroup) error {
	r.methodsCalled <- "UpsertRuleGroup"

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
