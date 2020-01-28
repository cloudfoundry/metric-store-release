package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/debug"
	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	sd_config "github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/notifier"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/rules"
	"github.com/prometheus/prometheus/storage"
)

type PromRuleManager struct {
	id                   string
	evaluationInterval   time.Duration
	promRuleManager      *rules.Manager
	promNotifierManager  *notifier.Manager
	promDiscoveryManager *discovery.Manager
	promRuleFile         string
	alertmanagerAddr     string
	log                  *logger.Logger
	metrics              debug.MetricRegistrar
	cancel               context.CancelFunc
}

func NewPromRuleManager(managerId, promRuleFile, alertmanagerAddr string, evaluationInterval time.Duration, store storage.Storage, engine *promql.Engine, log *logger.Logger, metrics debug.MetricRegistrar) *PromRuleManager {
	rulesManagerRegisterer := prometheus.WrapRegistererWith(
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
		QueryFunc:   wrappedEngineQueryFunc(engine, store),
		NotifyFunc:  sendAlerts(promNotifierManager),
		Context:     context.Background(),
		ExternalURL: &url.URL{},
		Logger:      rulesManagerLog,
		Registerer:  rulesManagerRegisterer,
	})

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	promDiscoveryManager := discovery.NewManager(
		ctx,
		rulesManagerLog,
		discovery.Name("notify"),
	)

	return &PromRuleManager{
		id:                   managerId,
		evaluationInterval:   evaluationInterval,
		promRuleManager:      promRuleManager,
		promNotifierManager:  promNotifierManager,
		promDiscoveryManager: promDiscoveryManager,
		promRuleFile:         promRuleFile,
		alertmanagerAddr:     alertmanagerAddr,
		log:                  rulesManagerLog,
		metrics:              metrics,
		cancel:               cancel,
	}
}

func (r *PromRuleManager) Start() error {
	err := r.Reload()
	if err != nil {
		return err
	}
	r.promRuleManager.Run()

	go r.promDiscoveryManager.Run()
	go r.promNotifierManager.Run(r.promDiscoveryManager.SyncCh())

	return nil
}

func (r *PromRuleManager) Stop() error {
	r.promRuleManager.Stop()
	r.promNotifierManager.Stop()
	r.cancel()

	return nil
}

func (r *PromRuleManager) Reload() error {
	err := r.promRuleManager.Update(r.evaluationInterval, []string{r.promRuleFile}, nil)
	if err != nil {
		return err
	}

	if r.alertmanagerAddr == "" {
		return nil
	}

	cfg := &config.Config{
		AlertingConfig: config.AlertingConfig{
			AlertmanagerConfigs: config.AlertmanagerConfigs{
				{
					ServiceDiscoveryConfig: sd_config.ServiceDiscoveryConfig{
						StaticConfigs: []*targetgroup.Group{
							{
								Targets: []model.LabelSet{
									{
										"__address__": model.LabelValue(r.alertmanagerAddr),
									},
								},
							},
						},
					},
					Scheme:     "http",
					Timeout:    10000000000,
					APIVersion: config.AlertmanagerAPIVersionV2,
				},
			},
		},
	}

	if err := r.promNotifierManager.ApplyConfig(cfg); err != nil {
		r.log.Fatal("error Applying the config", err)
		return err
	}

	discoveredConfig := make(map[string]sd_config.ServiceDiscoveryConfig)
	for i, v := range cfg.AlertingConfig.AlertmanagerConfigs {
		// AlertmanagerConfigs doesn't hold an unique identifier so we use the config hash as the identifier.
		_, err := json.Marshal(v)
		if err != nil {
			r.log.Fatal("error parsing alertmanager config", err)
			return err
		}
		discoveredConfig[fmt.Sprintf("config-%d", i)] = v.ServiceDiscoveryConfig
	}
	r.promDiscoveryManager.ApplyConfig(discoveredConfig)

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
