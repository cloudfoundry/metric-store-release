package rules

import (
	"encoding/json"
	"errors"
	"time"

	prom_versioned_api_client "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql/parser"
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

func ConvertAPIRulesToRule(apiRules prom_versioned_api_client.Rules) []rules.Rule {
	var rs []rules.Rule

	for _, apiRule := range apiRules {
		switch r := apiRule.(type) {
		case prom_versioned_api_client.AlertingRule:
			rs = append(rs, rules.NewAlertingRule(r.Name, nil, 0,
				nil, nil, nil,
				true, nil))
			//	vec parser.Expr, hold time.Duration,
			//	labels, annotations, externalLabels labels.Labels,
			//	restored bool, logger log.Logger,

		case prom_versioned_api_client.RecordingRule:
			expr, _ := parser.ParseExpr(r.Query)

			l2, _ := json.Marshal(r.Labels)
			ls := labels.Labels{}
			ls.UnmarshalJSON(l2)

			rule := rules.NewRecordingRule(r.Name, expr, ls)
			rule.SetHealth(rules.RuleHealth(r.Health))
			rule.SetLastError(errors.New(r.LastError))
			rs = append(rs, rule)
		}
	}

	return rs
}

// switch rule := r.(type) {
// case *rules.AlertingRule:
// 	if !returnAlerts {
// 		break
// 	}
// 	enrichedRule = alertingRule{
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
// 	}
// case *rules.RecordingRule:
// 	if !returnRecording {
// 		break
// 	}
// 	enrichedRule = recordingRule{
// 		Name:           rule.Name(),
// 		Query:          rule.Query().String(),
// 		Labels:         rule.Labels(),
// 		Health:         rule.Health(),
// 		LastError:      lastError,
// 		EvaluationTime: rule.GetEvaluationDuration().Seconds(),
// 		LastEvaluation: rule.GetEvaluationTimestamp(),
// 		Type:           "recording",
// 	}
