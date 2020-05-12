package api

import (
	"github.com/prometheus/prometheus/config"
	"net/http"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/prometheus/common/route"
)

type ReloadAPI struct {
	reloadConfig func()
	log          *logger.Logger
}

type ScrapeManager interface {
	ApplyConfig(cfg *config.Config) error
}

type DiscoveryAgent interface {
	ApplyScrapeConfig(configs []*config.ScrapeConfig)
}

func NewReloadAPI(reloadConfig func(), log *logger.Logger) *ReloadAPI {
	return &ReloadAPI{
		reloadConfig: reloadConfig,
		log:          log,
	}
}

func (api *ReloadAPI) Router() *route.Router {
	reloadAPIRouter := route.New()
	api.Register(reloadAPIRouter)

	return reloadAPIRouter
}

func (api *ReloadAPI) Register(r *route.Router) {
	r.Post("/reload", api.reload)
}

// TODO what if we get multiple concurrent reload requests?
func (api *ReloadAPI) reload(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	api.reloadConfig()
	w.WriteHeader(http.StatusOK)
}
