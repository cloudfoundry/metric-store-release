package discovery

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/go-kit/kit/log"
	"github.com/prometheus/prometheus/config"
	prom_discovery "github.com/prometheus/prometheus/discovery"
	sd_config "github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
)

type DiscoveryAgent struct {
	manager *prom_discovery.Manager
	name    string
	cancel  context.CancelFunc
}

func NewDiscoveryAgent(name string, l *logger.Logger) *DiscoveryAgent {
	discoveryCtxScrape, cancel := context.WithCancel(context.Background())

	manager := prom_discovery.NewManager(
		discoveryCtxScrape,
		log.With(l, "component", "discovery manager "+name),
		prom_discovery.Name(name),
	)

	return &DiscoveryAgent{
		manager: manager,
		name:    name,
		cancel:  cancel,
	}
}

func (d *DiscoveryAgent) ApplyScrapeConfig(configs []*config.ScrapeConfig) {
	discoveredConfig := make(map[string]sd_config.ServiceDiscoveryConfig)
	for _, v := range configs {
		discoveredConfig[v.JobName] = v.ServiceDiscoveryConfig
	}

	d.manager.ApplyConfig(discoveredConfig)
}

func (d *DiscoveryAgent) ApplyAlertmanagerConfig(configs config.AlertmanagerConfigs) error {
	discoveredConfig := make(map[string]sd_config.ServiceDiscoveryConfig)
	for i, v := range configs {
		// AlertmanagerConfigs doesn't hold an unique identifier so we use the config hash as the identifier.
		_, err := json.Marshal(v)
		if err != nil {
			return err
		}
		discoveredConfig[fmt.Sprintf("config-%d", i)] = v.ServiceDiscoveryConfig
	}

	d.manager.ApplyConfig(discoveredConfig)

	return nil
}

func (d *DiscoveryAgent) Start() {
	go func() {
		err := d.manager.Run()
		if err != nil {
			panic(err)
		}
	}()
}

func (d *DiscoveryAgent) SyncCh() <-chan map[string][]*targetgroup.Group {
	return d.manager.SyncCh()
}

func (d *DiscoveryAgent) Stop() {
	d.cancel()
}
