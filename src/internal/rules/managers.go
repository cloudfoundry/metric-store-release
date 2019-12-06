package rules

import (
	"context"
	"net/url"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/prometheus/prometheus/notifier"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/rules"
	"github.com/prometheus/prometheus/storage"
)

type RuleManagers struct {
	ruleManagers []*RuleManager
	store        storage.Storage
	engine       *promql.Engine
	log          *logger.Logger
	metrics      debug.MetricRegistrar
}

func NewRuleManagers(store storage.Storage, engine *promql.Engine, log *logger.Logger, metrics debug.MetricRegistrar) *RuleManagers {
	return &RuleManagers{
		store:   store,
		engine:  engine,
		log:     log,
		metrics: metrics,
	}
}

func (r *RuleManagers) Create(managerId, ruleFile, alertmanagerAddr string) {
	ruleManager := NewRuleManager(managerId, ruleFile, alertmanagerAddr, r.store, r.engine, r.log, r.metrics)
	r.add(ruleManager)
	ruleManager.Start()
}

func (r *RuleManagers) add(ruleManager *RuleManager) {
	r.ruleManagers = append(r.ruleManagers, ruleManager)
}

func (r *RuleManagers) Reload() {
	for _, ruleManager := range r.ruleManagers {
		ruleManager.Reload()
	}
}

func (r *RuleManagers) RuleGroups() []*rules.Group {
	var groups []*rules.Group

	for _, ruleManager := range r.ruleManagers {
		groups = append(groups, ruleManager.PromRuleManager.RuleGroups()...)
	}

	return groups
}

func (r *RuleManagers) AlertingRules() []*rules.AlertingRule {
	var alertingRules []*rules.AlertingRule

	for _, ruleManager := range r.ruleManagers {
		alertingRules = append(alertingRules, ruleManager.PromRuleManager.AlertingRules()...)
	}

	return alertingRules
}

func (r *RuleManagers) Alertmanagers() []*url.URL {
	var alertmanagers []*url.URL

	for _, ruleManager := range r.ruleManagers {
		alertmanagers = append(alertmanagers, ruleManager.PromNotifierManager.Alertmanagers()...)
	}

	return alertmanagers
}

func (r *RuleManagers) DroppedAlertmanagers() []*url.URL {
	var alertmanagers []*url.URL

	for _, ruleManager := range r.ruleManagers {
		alertmanagers = append(alertmanagers, ruleManager.PromNotifierManager.DroppedAlertmanagers()...)
	}

	return alertmanagers
}

func sendAlerts(s *notifier.Manager) rules.NotifyFunc {
	return func(ctx context.Context, expr string, alerts ...*rules.Alert) {
		var res []*notifier.Alert

		for _, alert := range alerts {
			a := &notifier.Alert{
				StartsAt:    alert.FiredAt,
				Labels:      alert.Labels,
				Annotations: alert.Annotations,
			}
			if !alert.ResolvedAt.IsZero() {
				a.EndsAt = alert.ResolvedAt
			} else {
				a.EndsAt = alert.ValidUntil
			}
			res = append(res, a)
		}

		if len(alerts) > 0 {
			s.Send(res...)
		}
	}
}

func EngineQueryFunc(engine *promql.Engine, q storage.Queryable) rules.QueryFunc {
	return func(ctx context.Context, qs string, t time.Time) (promql.Vector, error) {
		vector, err := rules.EngineQueryFunc(engine, q)(ctx, qs, t)
		if err != nil {
			return nil, err
		}

		samples := []promql.Sample{}
		for _, sample := range vector {
			samples = append(samples, promql.Sample{
				Point: promql.Point{
					T: transform.MillisecondsToNanoseconds(sample.T),
					V: sample.V,
				},
				Metric: sample.Metric,
			})
		}

		return promql.Vector(samples), nil
	}
}
