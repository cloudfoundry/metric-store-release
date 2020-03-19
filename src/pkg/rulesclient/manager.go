package rulesclient

import (
	prom_config "github.com/prometheus/prometheus/config"
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

type Manager interface {
	Id() string
	AlertManagers() *prom_config.AlertmanagerConfigs
}
