package rulesclient

import (
	"encoding/json"
	"io"

	"github.com/google/uuid"
	prom_config "github.com/prometheus/prometheus/config"
	"gopkg.in/yaml.v2"
	yamlj "sigs.k8s.io/yaml"
)

type managerData struct {
	Data manager `json:"data"`
}

type manager struct {
	Id            string `json:"id"`
	Alertmanagers json.RawMessage
}

type ManagerConfig struct {
	id            string
	alertmanagers *prom_config.AlertmanagerConfigs
}

func NewManagerConfig(id string, alertmanagers *prom_config.AlertmanagerConfigs) *ManagerConfig {
	return &ManagerConfig{
		id:            id,
		alertmanagers: alertmanagers,
	}
}

func (m *ManagerConfig) Id() string {
	return m.id
}

func (m *ManagerConfig) GenerateId() {
	m.id = uuid.New().String()
}

func (m *ManagerConfig) AlertManagers() *prom_config.AlertmanagerConfigs {
	return m.alertmanagers
}

func (m *ManagerConfig) Validate() error {
	return nil
}

func (m *ManagerConfig) ToJSON() ([]byte, error) {
	y, err := yaml.Marshal(m.AlertManagers())
	if err != nil {
		return nil, err
	}

	j, err := yamlj.YAMLToJSON(y)
	if err != nil {
		return nil, err
	}

	manager := manager{
		Id:            m.id,
		Alertmanagers: j,
	}
	data := managerData{
		Data: manager,
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func ManagerConfigFromJSON(body io.ReadCloser) (*ManagerConfig, error) {
	managerData := managerData{}

	err := json.NewDecoder(body).Decode(&managerData)
	if err != nil {
		return nil, err
	}

	var alertManagers prom_config.AlertmanagerConfigs
	if managerData.Data.Alertmanagers != nil {
		y, err := yamlj.JSONToYAML([]byte(string(managerData.Data.Alertmanagers)))
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(y, &alertManagers)
		if err != nil {
			return nil, err
		}
	}

	managerConfig := &ManagerConfig{
		id:            managerData.Data.Id,
		alertmanagers: &alertManagers,
	}

	return managerConfig, nil
}
