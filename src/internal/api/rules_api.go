package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	"github.com/go-kit/kit/log/level"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/common/route"
	"github.com/prometheus/prometheus/util/httputil"
)

type RulesAPI struct {
	ruleManager rules.RuleManager
	log         *logger.Logger
}

func NewRulesAPI(ruleManager rules.RuleManager, log *logger.Logger) *RulesAPI {
	return &RulesAPI{
		ruleManager: ruleManager,
		log:         log,
	}
}

type errorType string

type apiFuncResult struct {
	data interface{}
	err  *apiError
}

type apiFunc func(r *http.Request) apiFuncResult

type apiError struct {
	typ int
	err error
}

func (e *apiError) Error() string {
	return fmt.Sprintf("Error code %d: %s", e.typ, e.err)
}

func (api *RulesAPI) Router() *route.Router {
	rulesAPIRouter := route.New()
	api.Register(rulesAPIRouter)

	return rulesAPIRouter
}

func (api *RulesAPI) Register(r *route.Router) {
	wrap := func(f apiFunc) http.HandlerFunc {
		hf := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result := f(r)
			if result.err != nil {
				api.respondError(w, *result.err, result.data)
			} else if result.data == nil {
				api.respondNoData(w)
			} else {
				api.respond(w, result.data)
			}
		})

		return httputil.CompressionHandler{
			Handler: hf,
		}.ServeHTTP
	}
	r.Post("/manager", wrap(api.createManager))
	r.Post("/manager/:manager_id/group", wrap(api.createRuleGroup))
	r.Del("/manager/:manager_id", wrap(api.deleteManager))
}

func (api *RulesAPI) createManager(r *http.Request) apiFuncResult {
	defer r.Body.Close()

	var managerData rulesclient.ManagerData

	err := json.NewDecoder(r.Body).Decode(&managerData)
	if err != nil {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}

	if managerData.Data.Id == "" {
		managerData.Data.Id = uuid.New().String()
	}

	err = api.ruleManager.Create(managerData.Data.Id, managerData.Data.AlertManagerUrl)
	if err != nil {
		var returnErr *apiError

		switch err {
		case rules.ManagerExistsError:
			returnErr = &apiError{
				http.StatusConflict,
				fmt.Errorf("Could not create ruleManager, a ruleManager with name %s already exists", managerData.Data.Id),
			}
		default:
			returnErr = &apiError{
				http.StatusInternalServerError,
				err,
			}
		}

		return apiFuncResult{nil, returnErr}
	}

	return apiFuncResult{managerData, nil}
}

func (api *RulesAPI) deleteManager(r *http.Request) apiFuncResult {
	defer r.Body.Close()

	ctx := r.Context()
	managerId := route.Param(ctx, "manager_id")

	err := api.ruleManager.DeleteManager(managerId)
	if err != nil {
		return apiFuncResult{nil, &apiError{http.StatusNotFound, err}}
	}

	return apiFuncResult{}
}

func (api *RulesAPI) createRuleGroup(r *http.Request) apiFuncResult {
	defer r.Body.Close()

	ctx := r.Context()
	managerId := route.Param(ctx, "manager_id")

	var ruleGroupData rulesclient.RuleGroupData

	err := json.NewDecoder(r.Body).Decode(&ruleGroupData)
	if err != nil {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}

	if err = ruleGroupData.Data.Validate(); err != nil {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}

	err = api.ruleManager.UpsertRuleGroup(managerId, &ruleGroupData.Data)
	if err != nil {
		var returnErr *apiError

		switch err {
		case rules.ManagerNotExistsError:
			returnErr = &apiError{
				http.StatusBadRequest,
				err,
			}
		default:
			returnErr = &apiError{
				http.StatusInternalServerError,
				err,
			}
		}

		return apiFuncResult{nil, returnErr}
	}

	return apiFuncResult{ruleGroupData, nil}
}

func (api *RulesAPI) respond(w http.ResponseWriter, data interface{}) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	b, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if n, err := w.Write(b); err != nil {
		level.Error(api.log).Log("msg", "error writing response", "bytesWritten", n, "err", err)
	}
}

func (api *RulesAPI) respondNoData(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func (api *RulesAPI) respondError(w http.ResponseWriter, apiErr apiError, data interface{}) {
	w.WriteHeader(apiErr.typ)

	apiErrors := &rulesclient.ApiErrors{
		Errors: []rulesclient.ApiError{{
			Status: apiErr.typ,
			Title:  apiErr.err.Error(),
		}},
	}

	bytes, err := json.Marshal(apiErrors)
	if err != nil {
		return
	}
	w.Write(bytes)
}
