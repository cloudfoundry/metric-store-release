package rules

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/notifier"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/rules"
	"github.com/prometheus/prometheus/storage"
)

type PromRuleManagers struct {
	promRuleManagers   map[string]*PromRuleManager
	store              storage.Storage
	engine             *promql.Engine
	evaluationInterval time.Duration
	log                *logger.Logger
	metrics            metrics.Registrar
	queryTimeout       time.Duration
}

func NewRuleManagers(store storage.Storage, engine *promql.Engine, evaluationInterval time.Duration, log *logger.Logger, metrics metrics.Registrar, queryTimeout time.Duration) *PromRuleManagers {
	return &PromRuleManagers{
		store:              store,
		engine:             engine,
		evaluationInterval: evaluationInterval,
		log:                log,
		metrics:            metrics,
		promRuleManagers:   make(map[string]*PromRuleManager),
		queryTimeout:       queryTimeout,
	}
}

func (r *PromRuleManagers) Create(managerId, promRuleFile string, alertManagers *prom_config.AlertmanagerConfigs) error {
	promRuleManager := NewPromRuleManager(
		managerId,
		promRuleFile,
		alertManagers,
		r.evaluationInterval,
		r.store,
		r.engine,
		r.log,
		r.metrics,
		r.queryTimeout,
	)
	r.add(managerId, promRuleManager)
	return promRuleManager.Start()
}

func (r *PromRuleManagers) Delete(managerId string) error {
	promRuleManager, exists := r.promRuleManagers[managerId]
	if !exists {
		return ManagerNotExistsError
	}

	promRuleManager.Stop()
	delete(r.promRuleManagers, managerId)
	return nil
}

func (r *PromRuleManagers) DeleteAll() error {
	var deleteErrors []error
	for managerId := range r.promRuleManagers {
		err := r.Delete(managerId)
		if err != nil {
			deleteErrors = append(deleteErrors, err)
		}
	}
	if len(deleteErrors) > 0 {
		return fmt.Errorf("unable to delete %d of %d rule managers: %v", len(deleteErrors), len(r.promRuleManagers), deleteErrors)
	}

	return nil
}

func (r *PromRuleManagers) add(managerId string, promRuleManager *PromRuleManager) {
	r.promRuleManagers[managerId] = promRuleManager
}

func (r *PromRuleManagers) Reload() error {
	var err error

	for _, promRuleManager := range r.promRuleManagers {
		err = promRuleManager.Reload()
		if err != nil {
			return err
		}
	}

	return nil
}

// implements prometheus api v1 rulesRetriever interface
func (r *PromRuleManagers) RuleGroups() []*rules.Group {
	var groups []*rules.Group

	for _, promRuleManager := range r.promRuleManagers {
		groups = append(groups, promRuleManager.RuleGroups()...)
	}

	return groups
}

// implements prometheus api v1 rulesRetriever interface
func (r *PromRuleManagers) AlertingRules() []*rules.AlertingRule {
	var alertingRules []*rules.AlertingRule

	for _, promRuleManager := range r.promRuleManagers {
		alertingRules = append(alertingRules, promRuleManager.AlertingRules()...)
	}

	return alertingRules
}

// implements prometheus api v1 alertmanagerRetriever interface
func (r *PromRuleManagers) Alertmanagers() []*url.URL {
	alertmanagers := make(map[string]*url.URL)
	var uniqueAlertmanagers []*url.URL

	for _, promRuleManager := range r.promRuleManagers {
		for _, url := range promRuleManager.Alertmanagers() {
			alertmanagers[url.String()] = url
		}
	}

	for _, url := range alertmanagers {
		uniqueAlertmanagers = append(uniqueAlertmanagers, url)
	}

	return uniqueAlertmanagers
}

// implements prometheus api v1 alertmanagerRetriever interface
func (r *PromRuleManagers) DroppedAlertmanagers() []*url.URL {
	var alertmanagers []*url.URL

	for _, promRuleManager := range r.promRuleManagers {
		alertmanagers = append(alertmanagers, promRuleManager.DroppedAlertmanagers()...)
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

func wrappedEngineQueryFunc(engine *promql.Engine, q storage.Queryable, queryTimeout time.Duration) rules.QueryFunc {
	return func(ctx context.Context, qs string, t time.Time) (promql.Vector, error) {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, queryTimeout)
		defer cancel()

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
