package testing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"

	"github.com/prometheus/prometheus/notifier"
)

type AlertManagerSpy struct {
	server            *httptest.Server
	alertsReceived    *int64
	lastAlertReceived string
}

func NewAlertManagerSpy() *AlertManagerSpy {
	return &AlertManagerSpy{
		alertsReceived:    new(int64),
		lastAlertReceived: "",
	}
}

func (a *AlertManagerSpy) AlertsReceived() int64 {
	return atomic.LoadInt64(a.alertsReceived)
}

func (a *AlertManagerSpy) LastAlertReceived() string {
	return a.lastAlertReceived
}

func (a *AlertManagerSpy) receive(rw http.ResponseWriter, r *http.Request) {
	var receivedAlerts []*notifier.Alert
	defer r.Body.Close()

	json.NewDecoder(r.Body).Decode(&receivedAlerts)

	if len(receivedAlerts) > 0 {
		a.lastAlertReceived = receivedAlerts[len(receivedAlerts)-1].Name()
		atomic.AddInt64(a.alertsReceived, int64(len(receivedAlerts)))
	}
	rw.WriteHeader(http.StatusOK)
}

func (a *AlertManagerSpy) Start() {
	a.server = httptest.NewServer(http.HandlerFunc(a.receive))
}

func (a *AlertManagerSpy) Stop() {
	a.server.Close()
}

func (a *AlertManagerSpy) Addr() string {
	addr, _ := url.Parse(a.server.URL)
	return fmt.Sprintf("localhost:%s", addr.Port())
}
