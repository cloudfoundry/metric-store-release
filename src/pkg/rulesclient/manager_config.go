package rulesclient

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
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
	for _, alertmanager := range m.alertmanagers.ToMap() {
		err := validateCACert(alertmanager.HTTPClientConfig.TLSConfig.CAFile)
		if err != nil {
			return err
		}

		err = validateCertAndKey(
			alertmanager.HTTPClientConfig.TLSConfig.CertFile,
			alertmanager.HTTPClientConfig.TLSConfig.KeyFile,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateCACert(caCert string) error {
	if CertificateRegexp.MatchString(caCert) {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(caCert)) {
			return fmt.Errorf("unable to use specified CA cert %s", caCert)
		}
	}
	return nil
}

func validateCertAndKey(cert, key string) error {
	if CertificateRegexp.MatchString(cert) || CertificateRegexp.MatchString(key) {
		_, err := tls.X509KeyPair([]byte(cert), []byte(key))
		return err
	}
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
