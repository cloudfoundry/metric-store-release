package rules

import (
	"context"
	html_template "html/template"
	"net/url"
	"time"

	prom_versioned_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/rules"
)

func ConvertAPIRuleGroupToGroup(ruleGroups []prom_versioned_api_client.RuleGroup) []*rules.Group {
	var groups []*rules.Group

	for _, apiRuleGroup := range ruleGroups {
		apiRuleGroupOptions := rules.GroupOptions{
			Name:     apiRuleGroup.Name,
			File:     apiRuleGroup.File,
			Interval: time.Duration(apiRuleGroup.Interval) * time.Second,
			Rules:    ConvertAPIRulesToRule(apiRuleGroup.Rules),
			Opts:     &rules.ManagerOptions{},
		}
		group := rules.NewGroup(apiRuleGroupOptions)
		groups = append(groups, group)
	}

	return groups
}

// NEEDS to fulfill api
// Name:           grp.Name(),
// File:           grp.File(),
// Interval:       grp.Interval().Seconds(),
// Rules:          []rule{},
// EvaluationTime: grp.GetEvaluationDuration().Seconds(),
// LastEvaluation: grp.GetEvaluationTimestamp(),

// grp.Rules()
// -> r.Type
// case *rules.AlertingRule:
// 		State:          rule.State().String(),
// 		Name:           rule.Name(),
// 		Query:          rule.Query().String(),
// 		Duration:       rule.HoldDuration().Seconds(),
// 		Labels:         rule.Labels(),
// 		Annotations:    rule.Annotations(),
// 		Alerts:         rulesAlertsToAPIAlerts(rule.ActiveAlerts()),
// 		Health:         rule.Health(),
// 		LastError:      lastError,
// 		EvaluationTime: rule.GetEvaluationDuration().Seconds(),
// 		LastEvaluation: rule.GetEvaluationTimestamp(),
// 		Type:           "alerting",
// case *rules.RecordingRule:
// 		Name:           rule.Name(),
// 		Query:          rule.Query().String(),
// 		Labels:         rule.Labels(),
// 		Health:         rule.Health(),
// 		LastError:      lastError,
// 		EvaluationTime: rule.GetEvaluationDuration().Seconds(),
// 		LastEvaluation: rule.GetEvaluationTimestamp(),
// 		Type:           "recording",

type conversionRule struct {
	name string
	// Query       string
	// Duration    float64
	// Labels      model.LabelSet
	// Annotations model.LabelSet
	// Alerts      []*prom_versioned_api_client.Alert
	// Health      prom_versioned_api_client.RuleHealth
	// LastError   string
}

// func NewConversionRule(name string) conversionRule {
// 	return conversionRule{
// }

func (c conversionRule) Name() string {
	return c.name
}

// Labels of the rule.
func (c conversionRule) Labels() labels.Labels {
	panic("not implemented") // TODO: Implement
}

// eval evaluates the rule, including any associated recording or alerting actions.
func (c conversionRule) Eval(_ context.Context, _ time.Time, _ rules.QueryFunc, _ *url.URL) (promql.Vector, error) {
	panic("not implemented") // TODO: Implement
}

// String returns a human-readable string representation of the rule.
func (c conversionRule) String() string {
	panic("not implemented") // TODO: Implement
}

// SetLastErr sets the current error experienced by the rule.
func (c conversionRule) SetLastError(_ error) {
	panic("not implemented") // TODO: Implement
}

// LastErr returns the last error experienced by the rule.
func (c conversionRule) LastError() error {
	panic("not implemented") // TODO: Implement
}

// SetHealth sets the current health of the rule.
func (c conversionRule) SetHealth(_ rules.RuleHealth) {
	panic("not implemented") // TODO: Implement
}

// Health returns the current health of the rule.
func (c conversionRule) Health() rules.RuleHealth {
	panic("not implemented") // TODO: Implement
}

func (c conversionRule) SetEvaluationDuration(_ time.Duration) {
	panic("not implemented") // TODO: Implement
}

// GetEvaluationDuration returns last evaluation duration.
// NOTE: Used dynamically by rules.html template.
func (c conversionRule) GetEvaluationDuration() time.Duration {
	panic("not implemented") // TODO: Implement
}

func (c conversionRule) SetEvaluationTimestamp(_ time.Time) {
	panic("not implemented") // TODO: Implement
}

// GetEvaluationTimestamp returns last evaluation timestamp.
// NOTE: Used dynamically by rules.html template.
func (c conversionRule) GetEvaluationTimestamp() time.Time {
	panic("not implemented") // TODO: Implement
}

// HTMLSnippet returns a human-readable string representation of the rule,
// decorated with HTML elements for use the web frontend.
func (c conversionRule) HTMLSnippet(pathPrefix string) html_template.HTML {
	panic("not implemented") // TODO: Implement
}

func ConvertAPIRulesToRule(apiRules prom_versioned_api_client.Rules) []rules.Rule {
	var rules []rules.Rule

	for _, apiRule := range apiRules {
		switch r := apiRule.(type) {
		case prom_versioned_api_client.RecordingRule:
			rules = append(rules, conversionRule{
				name: r.Name,
			})
		}
	}

	return rules
}
