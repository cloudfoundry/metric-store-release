package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/go-kit/kit/log/level"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/common/route"
	"github.com/prometheus/prometheus/util/httputil"
)

type RulesAPI struct {
	rulesStoragePath string
	log              *logger.Logger
}

func NewRulesAPI(rulesStoragePath string, log *logger.Logger) *RulesAPI {
	return &RulesAPI{
		rulesStoragePath: rulesStoragePath,
		log:              log,
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

	managerFile := path.Join(api.rulesStoragePath, body.Data.Id)
	_, err = os.Stat(managerFile)

	if err != nil && !os.IsNotExist(err) {
		return apiFuncResult{nil, &apiError{http.StatusInternalServerError, err}}
	}

	_, err = os.OpenFile(managerFile, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if os.IsExist(err) {
		return apiFuncResult{
			nil,
			&apiError{
				http.StatusConflict,
				fmt.Errorf("Could not create ruleManager, a ruleManager with name %s already exists", body.Data.Id),
			},
		}
	}

	return apiFuncResult{body, nil}
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
