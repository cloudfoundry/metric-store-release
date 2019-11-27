package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
	"github.com/go-kit/kit/log/level"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/route"
	"github.com/prometheus/prometheus/pkg/rulefmt"
	"github.com/prometheus/prometheus/util/httputil"
	"gopkg.in/yaml.v2"
)

type RulesAPI struct {
	rulesStoragePath   string
	evaluationInterval time.Duration
	log                *logger.Logger
}

func NewRulesAPI(rulesStoragePath string, evaluationInterval time.Duration, log *logger.Logger) *RulesAPI {
	return &RulesAPI{
		rulesStoragePath:   rulesStoragePath,
		evaluationInterval: evaluationInterval,
		log:                log,
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
			} else if result.data != nil {
				api.respond(w, result.data)
			}
		})

		return httputil.CompressionHandler{
			Handler: hf,
		}.ServeHTTP
	}

	r.Post("/manager", wrap(api.createManager))
	r.Post("/manager/:manager_id/group", wrap(api.createRuleGroup))
}

func (api *RulesAPI) createManager(r *http.Request) apiFuncResult {
	defer r.Body.Close()

	var body struct {
		Data struct {
			Id string `json:"id"`
		} `json:"data"`
	}

	var err error
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}

	if body.Data.Id == "" {
		body.Data.Id = uuid.New().String()
	}

	managerFile := api.rulesFileForManager(body.Data.Id)
	file, err := os.OpenFile(managerFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.ModePerm)
	if os.IsExist(err) {
		return apiFuncResult{
			nil,
			&apiError{
				http.StatusConflict,
				fmt.Errorf("Could not create ruleManager, a ruleManager with name %s already exists", body.Data.Id),
			},
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return apiFuncResult{nil, &apiError{http.StatusInternalServerError, err}}
	}
	defer file.Close()

	_, err = file.Write([]byte("groups: []"))
	if err != nil {
		return apiFuncResult{nil, &apiError{http.StatusInternalServerError, err}}
	}
	return apiFuncResult{body, nil}
}

func (api *RulesAPI) createRuleGroup(r *http.Request) apiFuncResult {
	defer r.Body.Close()

	ctx := r.Context()
	managerId := route.Param(ctx, "manager_id")

	var err error
	var body struct {
		Data struct {
			Name     string        `json:"name"`
			Interval string        `json:"interval"`
			Rules    []interface{} `json:"rules"`
		} `json:"data"`
	}

	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}
	exists, err := api.rulesManagerExists(managerId)
	if err != nil {
		return apiFuncResult{nil, &apiError{http.StatusInternalServerError, err}}
	}
	if !exists {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}

	if body.Data.Name == "" {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}

	var duration model.Duration
	if body.Data.Interval == "" {
		duration = model.Duration(api.evaluationInterval)
		body.Data.Interval = api.evaluationInterval.String()
	} else {
		duration, err = model.ParseDuration(body.Data.Interval)
	}
	if err != nil {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}

	var rules []rulefmt.Rule
	rulesBytes, err := json.Marshal(body.Data.Rules)
	if err != nil {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}
	err = json.Unmarshal(rulesBytes, &rules)
	if err != nil {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}

	if len(body.Data.Rules) == 0 {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}

	ruleGroup := rulefmt.RuleGroup{
		Name:     body.Data.Name,
		Interval: duration,
		Rules:    rules,
	}
	ruleGroups := rulefmt.RuleGroups{
		Groups: []rulefmt.RuleGroup{
			ruleGroup,
		},
	}

	if errs := ruleGroups.Validate(); len(errs) != 0 {
		return apiFuncResult{nil, &apiError{http.StatusBadRequest, err}}
	}

	err = api.writeRuleToFile(managerId, ruleGroup)
	if err != nil {
		return apiFuncResult{nil, &apiError{http.StatusInternalServerError, err}}
	}
	return apiFuncResult{body, nil}
}

func (api *RulesAPI) rulesManagerExists(managerId string) (bool, error) {
	managerFile := api.rulesFileForManager(managerId)
	_, err := os.Stat(managerFile)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (api *RulesAPI) writeRuleToFile(managerId string, ruleGroup rulefmt.RuleGroup) error {
	var err error

	rulesFile := api.rulesFileForManager(managerId)
	bytes, err := ioutil.ReadFile(rulesFile)
	var rgs rulefmt.RuleGroups
	err = yaml.Unmarshal(bytes, &rgs)
	if err != nil {
		return err
	}
	rgs.Groups = append(rgs.Groups, ruleGroup)
	outBytes, err := yaml.Marshal(rgs)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(rulesFile, outBytes, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (api *RulesAPI) rulesFileForManager(managerId string) string {
	return path.Join(api.rulesStoragePath, managerId)
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

func (api *RulesAPI) respondError(w http.ResponseWriter, apiErr apiError, data interface{}) {
	w.WriteHeader(apiErr.typ)
	bytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	w.Write(bytes)
}
