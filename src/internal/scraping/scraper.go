package scraping

import (
	"io/ioutil"
	"path/filepath"

	"github.com/go-kit/kit/log"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"

	"github.com/cloudfoundry/metric-store-release/src/internal/discovery"
	"github.com/cloudfoundry/metric-store-release/src/internal/routing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

type Scraper struct {
	log                       *logger.Logger
	discoveryAgent            *discovery.DiscoveryAgent
	scrapeManager             *scrape.Manager
	routingTable              *routing.RoutingTable
	configFile                string
	additionalScrapeConfigDir string
}

func New(scrapeConfigFile, additionalScrapeConfigDir string, log *logger.Logger, table *routing.RoutingTable) *Scraper {
	return &Scraper{
		configFile:                scrapeConfigFile,
		additionalScrapeConfigDir: additionalScrapeConfigDir,
		log:                       log,
		routingTable:              table,
	}
}

func (store *Scraper) Run(storage storage.Appendable) {
	// TODO refactor this so the control flow is less weird
	// note that LoadConfigs is passed to the reload api
	// RS & JG 05/12/2020
	store.scrapeManager = scrape.NewManager(nil, log.With(store.log, "component",
		"scrape manager"), storage)
	store.discoveryAgent = discovery.NewDiscoveryAgent("scrape", store.log)
	store.LoadConfigs()

	store.discoveryAgent.Start()
	go func(discoveryAgent *discovery.DiscoveryAgent) {
		err := store.scrapeManager.Run(discoveryAgent.SyncCh())
		if err != nil {
			panic(err)
		}
	}(store.discoveryAgent)
}

func (store *Scraper) LoadConfigs() {
	promConfig := &prom_config.Config{}

	if store.configFile != "" {
		var err error
		store.log.Debug("Adding base scrape config from path: " + store.configFile)
		promConfig, err = prom_config.LoadFile(store.configFile, false,
			false, store.log)
		if err != nil {
			panic(err)
		}
	}

	scrapeConfigs := store.filterScrapeConfigs(store.unfilteredScrapeConfigs(promConfig.ScrapeConfigs))

	promConfig.ScrapeConfigs = scrapeConfigs
	store.discoveryAgent.ApplyScrapeConfig(scrapeConfigs)
	err := store.scrapeManager.ApplyConfig(promConfig)
	if err != nil {
		store.log.Error("could not apply scrape config", err)
	}
}

func (store *Scraper) unfilteredScrapeConfigs(existing []*prom_config.ScrapeConfig) []*prom_config.ScrapeConfig {
	scrapeConfigs := existing
	if store.additionalScrapeConfigDir != "" {
		store.log.Debug("Adding additional scrape configs from path: " + store.additionalScrapeConfigDir)

		fileInfos, err := ioutil.ReadDir(store.additionalScrapeConfigDir)
		if err != nil {
			store.log.Error("Could not read files from additional scrape configs dir: "+store.additionalScrapeConfigDir, err)
			panic(err)
		}

		for _, fileInfo := range fileInfos {
			if !fileInfo.IsDir() {
				store.log.Debug("Found additional scrape config file " + fileInfo.Name())
				additionalScrapeConfig, err := prom_config.LoadFile(filepath.Join(store.additionalScrapeConfigDir,
					fileInfo.Name()), false, false, store.log)
				if err != nil {
					store.log.Error("Could not parse scrape config from path "+fileInfo.Name(), err)
				}
				scrapeConfigs = append(scrapeConfigs, additionalScrapeConfig.ScrapeConfigs...)
			}
		}
	}

	return scrapeConfigs
}

func (store *Scraper) filterScrapeConfigs(configs []*prom_config.ScrapeConfig) []*prom_config.ScrapeConfig {
	var filtered []*prom_config.ScrapeConfig

	for _, config := range configs {
		if store.routingTable.IsLocal(config.JobName) {
			filtered = append(filtered, config)
		}
	}

	return filtered
}
