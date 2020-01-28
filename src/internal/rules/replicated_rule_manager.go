package rules

import (
	"crypto/tls"
	"net/url"

	"github.com/cloudfoundry/metric-store-release/src/internal/routing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	"github.com/prometheus/prometheus/rules"
)

type ReplicatedRuleManager struct {
	localRuleManager RuleManager
	localIndex       int
	addrs            []string
	lookup           routing.Lookup
	ruleManagers     []RuleManager
	tlsConfig        *tls.Config
}

func NewReplicatedRuleManager(localRuleManager RuleManager, localIndex int, addrs []string, replicationFactor uint, tlsConfig *tls.Config) *ReplicatedRuleManager {
	replicatedRuleManager := &ReplicatedRuleManager{
		localRuleManager: localRuleManager,
		localIndex:       localIndex,
		addrs:            addrs,
		ruleManagers:     make([]RuleManager, len(addrs)),
		tlsConfig:        tlsConfig,
	}

	routingTable, _ := routing.NewRoutingTable(addrs, replicationFactor)
	replicatedRuleManager.lookup = routingTable.Lookup

	replicatedRuleManager.createRuleManagers()

	return replicatedRuleManager
}

func (r *ReplicatedRuleManager) Create(managerId, alertmanagerAddr string) error {
	var err error

	for _, nodeIndex := range r.lookup(managerId) {
		err = r.ruleManagers[nodeIndex].Create(managerId, alertmanagerAddr)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ReplicatedRuleManager) DeleteManager(managerId string) error {
	var err error

	for _, nodeIndex := range r.lookup(managerId) {
		err = r.ruleManagers[nodeIndex].DeleteManager(managerId)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ReplicatedRuleManager) UpsertRuleGroup(managerId string, ruleGroup *rulesclient.RuleGroup) error {
	var err error

	for _, nodeIndex := range r.lookup(managerId) {
		err = r.ruleManagers[nodeIndex].UpsertRuleGroup(managerId, ruleGroup)
		if err != nil {
			return err
		}
	}

	return nil
}

// TODO: dedup rules
// used for /api/v1/rules
func (r *ReplicatedRuleManager) RuleGroups() []*rules.Group {
	var ruleGroups []*rules.Group

	for _, ruleManager := range r.ruleManagers {
		ruleGroups = append(ruleGroups, ruleManager.RuleGroups()...)
	}

	return ruleGroups
}

// TODO: dedup alerts
// used for /api/v1/alerts
func (r *ReplicatedRuleManager) AlertingRules() []*rules.AlertingRule {
	var alertingRules []*rules.AlertingRule

	for _, ruleManager := range r.ruleManagers {
		alertingRules = append(alertingRules, ruleManager.AlertingRules()...)
	}

	return alertingRules
}

func (r *ReplicatedRuleManager) Alertmanagers() []*url.URL {
	urlSet := make(map[string]*url.URL)

	for _, ruleManager := range r.ruleManagers {
		for _, alertManager := range ruleManager.Alertmanagers() {
			urlSet[alertManager.String()] = alertManager
		}
	}

	var urls []*url.URL
	for _, alertmanagerUrl := range urlSet {
		urls = append(urls, alertmanagerUrl)
	}

	return urls
}

func (r *ReplicatedRuleManager) DroppedAlertmanagers() []*url.URL {
	urlSet := make(map[string]*url.URL)

	for _, ruleManager := range r.ruleManagers {
		for _, alertManager := range ruleManager.DroppedAlertmanagers() {
			urlSet[alertManager.String()] = alertManager
		}
	}

	var urls []*url.URL
	for _, alertmanagerUrl := range urlSet {
		urls = append(urls, alertmanagerUrl)
	}

	return urls
}

func (r *ReplicatedRuleManager) createRuleManagers() error {
	for nodeIndex, addr := range r.addrs {
		if nodeIndex != r.localIndex {
			r.ruleManagers[nodeIndex] = NewRemoteRuleManager(addr, r.tlsConfig)
			continue
		}

		r.ruleManagers[nodeIndex] = r.localRuleManager
	}

	return nil
}
