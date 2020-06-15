package rules

import (
	"context"
	"net/url"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/discovery"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus/prometheus/config"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/notifier"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/rules"
	"github.com/prometheus/prometheus/storage"
)

type PromRuleManager struct {
	id                     string
	evaluationInterval     time.Duration
	promRuleManager        *rules.Manager
	promNotifierManager    *notifier.Manager
	promDiscoveryManager   *discovery.DiscoveryAgent
	promRuleFile           string
	alertManagers          *prom_config.AlertmanagerConfigs
	log                    *logger.Logger
	metrics                metrics.Registrar
	rulesManagerRegisterer *Registerer
}

func NewPromRuleManager(managerId, promRuleFile string, alertManagers *prom_config.AlertmanagerConfigs, evaluationInterval time.Duration, store storage.Storage, engine *promql.Engine, log *logger.Logger, metrics metrics.Registrar, queryTimeout time.Duration) *PromRuleManager {
	rulesManagerRegisterer := NewRegisterer(
		prometheus.Labels{"manager_id": managerId},
		metrics.Registerer(),
	)
	rulesManagerLog := log.NamedLog(managerId)

	options := &notifier.Options{
		QueueCapacity: 10,
		Registerer:    rulesManagerRegisterer,
	}

	promNotifierManager := notifier.NewManager(options, log)
	promRuleManager := rules.NewManager(&rules.ManagerOptions{
		Appendable:  store,
		TSDB:        store,
		QueryFunc:   wrappedEngineQueryFunc(engine, store, queryTimeout),
		NotifyFunc:  sendAlerts(promNotifierManager),
		Context:     context.Background(),
		ExternalURL: &url.URL{},
		Logger:      rulesManagerLog,
		Registerer:  rulesManagerRegisterer,
	})

	promDiscoveryManager := discovery.NewDiscoveryAgent(
		"notify",
		rulesManagerLog,
	)

	return &PromRuleManager{
		id:                     managerId,
		evaluationInterval:     evaluationInterval,
		promRuleManager:        promRuleManager,
		rulesManagerRegisterer: rulesManagerRegisterer,
		promNotifierManager:    promNotifierManager,
		promDiscoveryManager:   promDiscoveryManager,
		promRuleFile:           promRuleFile,
		alertManagers:          alertManagers,
		log:                    rulesManagerLog,
		metrics:                metrics,
	}
}

func (r *PromRuleManager) Start() error {
	err := r.Reload()
	if err != nil {
		return err
	}
	r.promRuleManager.Run()
	r.promDiscoveryManager.Start()

	go r.promNotifierManager.Run(r.promDiscoveryManager.SyncCh())

	return nil
}

func (r *PromRuleManager) Stop() error {
	r.promRuleManager.Stop()
	r.promNotifierManager.Stop()
	r.promDiscoveryManager.Stop()
	r.rulesManagerRegisterer.UnregisterAll()
	return nil
}

func (r *PromRuleManager) Reload() error {
	err := r.promRuleManager.Update(r.evaluationInterval, []string{r.promRuleFile}, nil)
	if err != nil {
		return err
	}

	if r.alertManagers == nil {
		return nil
	}
	cfg := &config.Config{
		AlertingConfig: config.AlertingConfig{
			AlertmanagerConfigs: *r.alertManagers,
		},
	}

	if err := r.promNotifierManager.ApplyConfig(cfg); err != nil {
		r.log.Error("error Applying the config", err)
		return err
	}

	err = r.promDiscoveryManager.ApplyAlertmanagerConfig(cfg.AlertingConfig.AlertmanagerConfigs)
	if err != nil {
		r.log.Error("error parsing alertmanager config", err)
		return err
	}

	return nil
}

func (r *PromRuleManager) RuleGroups() []*rules.Group {
	return r.promRuleManager.RuleGroups()
}

func (r *PromRuleManager) AlertingRules() []*rules.AlertingRule {
	return r.promRuleManager.AlertingRules()
}

func (r *PromRuleManager) Alertmanagers() []*url.URL {
	return r.promNotifierManager.Alertmanagers()
}

func (r *PromRuleManager) DroppedAlertmanagers() []*url.URL {
	return r.promNotifierManager.DroppedAlertmanagers()
}
