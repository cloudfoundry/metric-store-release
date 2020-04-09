package rules

import (
	"fmt"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/pkg/rulefmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

type ManagerStoreError string

func (e ManagerStoreError) Error() string { return string(e) }

type RuleManagerFiles struct {
	rulesStoragePath string
}

func NewRuleManagerFiles(rulesStoragePath string) *RuleManagerFiles {
	return &RuleManagerFiles{rulesStoragePath: rulesStoragePath}
}

func (f *RuleManagerFiles) Create(managerId string, alertManagers *prom_config.AlertmanagerConfigs) (string, error) {
	exists, err := f.rulesManagerExists(managerId)
	if err != nil {
		return "", err
	}
	if exists {
		return "", ManagerExistsError
	}

	err = os.Mkdir(f.ruleManagerDirectoryPath(managerId), os.ModePerm)
	if err != nil {
		return "", err
	}

	err = f.writeRules(managerId, nil)
	if err != nil {
		return "", err
	}
	err = f.writeCerts(managerId, alertManagers)
	if err != nil {
		return "", err
	}
	err = f.writeAlertManager(managerId, alertManagers)
	if err != nil {
		return "", err
	}

	return f.rulesFilePath(managerId), err
}

func (f *RuleManagerFiles) Delete(managerId string) error {
	exists, err := f.rulesManagerExists(managerId)
	if err != nil {
		return err
	}
	if !exists {
		return ManagerNotExistsError
	}

	return f.remove(managerId)
}

func (f *RuleManagerFiles) Load(managerId string) (string, *prom_config.AlertmanagerConfigs, error) {
	rulesFilePath := f.rulesFilePath(managerId)

	alertManagerFilePath := f.alertManagerFilePath(managerId)

	_, err := os.Stat(alertManagerFilePath)
	if os.IsNotExist(err) {
		return rulesFilePath, nil, nil
	}

	data, err := ioutil.ReadFile(alertManagerFilePath)
	if err != nil {
		return "", nil, err
	}

	var alertManagers prom_config.AlertmanagerConfigs
	err = yaml.Unmarshal(data, &alertManagers)
	if err != nil {
		return "", nil, err
	}

	return rulesFilePath, &alertManagers, nil
}

func (f *RuleManagerFiles) UpsertRuleGroup(managerId string, ruleGroup *rulefmt.RuleGroup) error {
	exists, err := f.rulesManagerExists(managerId)
	if err != nil {
		return err
	}
	if !exists {
		return ManagerNotExistsError
	}

	return f.writeRules(managerId, ruleGroup)
}

func (f *RuleManagerFiles) writeAlertManager(managerId string, alertmanagers *prom_config.AlertmanagerConfigs) error {
	alertManagerFilePath := f.alertManagerFilePath(managerId)

	outBytes, err := yaml.Marshal(alertmanagers)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(alertManagerFilePath, outBytes, os.ModePerm)
}

func (f *RuleManagerFiles) writeRules(managerId string, ruleGroup *rulefmt.RuleGroup) error {
	managerFilePath := f.rulesFilePath(managerId)

	ruleGroups := rulefmt.RuleGroups{}
	if ruleGroup != nil {
		ruleGroups.Groups = []rulefmt.RuleGroup{*ruleGroup}
	}

	outBytes, err := yaml.Marshal(ruleGroups)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(managerFilePath, outBytes, os.ModePerm)
}

func (f *RuleManagerFiles) writeCerts(managerId string, alertmanagers *prom_config.AlertmanagerConfigs) error {
	if alertmanagers == nil {
		return nil
	}
	var err error

	for index, alertmanager := range alertmanagers.ToMap() {
		filePath := f.caCertFilePath(managerId, index)
		alertmanager.HTTPClientConfig.TLSConfig.CAFile, err = f.extractCertificate(filePath, alertmanager.HTTPClientConfig.TLSConfig.CAFile)
		if err != nil {
			return err
		}

		filePath = f.certFilePath(managerId, index)
		alertmanager.HTTPClientConfig.TLSConfig.CertFile, err = f.extractCertificate(filePath, alertmanager.HTTPClientConfig.TLSConfig.CertFile)
		if err != nil {
			return err
		}

		filePath = f.keyFilePath(managerId, index)
		alertmanager.HTTPClientConfig.TLSConfig.KeyFile, err = f.extractCertificate(filePath, alertmanager.HTTPClientConfig.TLSConfig.KeyFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *RuleManagerFiles) extractCertificate(path, cert string) (string, error) {
	if rulesclient.CertificateRegexp.MatchString(cert) {
		err := ioutil.WriteFile(path, []byte(cert), os.ModePerm)
		if err != nil {
			return cert, err
		}

		return path, nil
	}

	return cert, nil
}

func (f *RuleManagerFiles) remove(managerId string) error {
	managerDirectoryPath := f.ruleManagerDirectoryPath(managerId)
	return os.RemoveAll(managerDirectoryPath)
}

func (f *RuleManagerFiles) rulesManagerExists(managerId string) (bool, error) {
	managerDirectoryPath := f.ruleManagerDirectoryPath(managerId)
	_, err := os.Stat(managerDirectoryPath)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (f *RuleManagerFiles) ruleManagerDirectoryPath(managerId string) string {
	return filepath.Join(f.rulesStoragePath, managerId)
}

func (f *RuleManagerFiles) rulesFilePath(managerId string) string {
	return filepath.Join(f.ruleManagerDirectoryPath(managerId), "rules.yml")
}

func (f *RuleManagerFiles) alertManagerFilePath(managerId string) string {
	return filepath.Join(f.ruleManagerDirectoryPath(managerId), "alertmanager.yml")
}

func (f *RuleManagerFiles) caCertFilePath(managerId, index string) string {
	return filepath.Join(f.ruleManagerDirectoryPath(managerId), fmt.Sprintf("alertmanager-%s-ca.crt", index))
}

func (f *RuleManagerFiles) certFilePath(managerId, index string) string {
	return filepath.Join(f.ruleManagerDirectoryPath(managerId), fmt.Sprintf("alertmanager-%s.crt", index))
}

func (f *RuleManagerFiles) keyFilePath(managerId, index string) string {
	return filepath.Join(f.ruleManagerDirectoryPath(managerId), fmt.Sprintf("alertmanager-%s.key", index))
}
