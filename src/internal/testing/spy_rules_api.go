package testing

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	"github.com/gorilla/mux"
)

type RulesApiSpy struct {
	server           *http.Server
	tlsConfig        *tls.Config
	apiErrors        chan *RulesApiHttpError
	requestsReceived *int64
}

type RulesApiHttpError struct {
	Status int
	Title  string
}

type RulesApiResponse struct {
	Errors []RulesApiError `json:"errors"`
}

type RulesApiError struct {
	Title string `json:"title"`
}

func NewRulesApiSpy(tlsConfig *tls.Config) (*RulesApiSpy, error) {
	return &RulesApiSpy{
		tlsConfig:        tlsConfig,
		requestsReceived: new(int64),
		apiErrors:        make(chan *RulesApiHttpError, 1),
	}, nil
}

func (a *RulesApiSpy) RequestsReceived() int {
	return int(atomic.LoadInt64(a.requestsReceived))
}

func (a *RulesApiSpy) NextRequestError(err *RulesApiHttpError) {
	a.apiErrors <- err
}

func (a *RulesApiSpy) getNextError() *RulesApiHttpError {
	select {
	case err := <-a.apiErrors:
		return err
	default:
		return nil
	}
}

func (a *RulesApiSpy) createManager(rw http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body, _ := ioutil.ReadAll(r.Body)

	atomic.AddInt64(a.requestsReceived, 1)

	var receivedManagerData rulesclient.ManagerData
	json.Unmarshal(body, &receivedManagerData)

	if !a.writeError(rw) {
		rw.WriteHeader(http.StatusCreated)
		rw.Write(body)
	}
}

func (a *RulesApiSpy) writeError(rw http.ResponseWriter) bool {
	apiErr := a.getNextError()

	if apiErr == nil {
		return false
	}

	rw.WriteHeader(apiErr.Status)

	errors := []RulesApiError{
		RulesApiError{Title: apiErr.Title},
	}
	apiResponse := RulesApiResponse{
		Errors: errors,
	}
	json, err := json.Marshal(apiResponse)
	if err != nil {
		panic("Unable to marshal test data")
	}

	rw.Write([]byte(json))

	return true
}

func (a *RulesApiSpy) upsertGroup(rw http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	atomic.AddInt64(a.requestsReceived, 1)

	body, _ := ioutil.ReadAll(r.Body)

	if !a.writeError(rw) {
		var receivedGroupData rulesclient.RuleGroupData
		json.Unmarshal(body, &receivedGroupData)

		rw.WriteHeader(http.StatusCreated)
		rw.Write(body)
	}
}

func (a *RulesApiSpy) Start() error {
	insecureConnection, err := net.Listen("tcp", ":0")
	if err != nil {
		return err
	}
	secureConnection := tls.NewListener(insecureConnection, a.tlsConfig)

	mux := mux.NewRouter()
	mux.HandleFunc("/rules/manager", a.createManager)
	mux.HandleFunc("/rules/manager/{manager_id}/group", a.upsertGroup)
	a.server = &http.Server{Handler: mux, Addr: secureConnection.Addr().String()}

	go a.server.Serve(secureConnection)

	return nil
}

func (a *RulesApiSpy) Stop() {
	a.server.Close()
}

func (a *RulesApiSpy) Addr() string {
	return a.server.Addr
}
