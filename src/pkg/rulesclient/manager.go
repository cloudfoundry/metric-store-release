package rulesclient

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/validate"
)

type ApiErrors struct {
	Errors []ApiError `json:"errors"`
}

type ApiError struct {
	Status int    `json:"status"`
	Title  string `json:"title"`
}

func (a *ApiError) Error() string {
	return a.Title
}

type ManagerData struct {
	Data Manager `json:"data"`
}

type Manager struct {
	Id              string `json:"id"`
	AlertManagerUrl string `json:"alertmanager_url"`
}

func (m *Manager) Validate() error {
	return validate.AlertmanagerUrl(m.AlertManagerUrl)
}
