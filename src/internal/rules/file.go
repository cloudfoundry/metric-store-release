package rules

import (
	"bufio"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/prometheus/prometheus/pkg/rulefmt"
	"gopkg.in/yaml.v2"
)

type ManagerStoreError string

func (e ManagerStoreError) Error() string { return string(e) }

const (
	ManagerExistsError    = ManagerStoreError("ManagerExistsError")
	ManagerNotExistsError = ManagerStoreError("ManagerNotExistsError")

	alertManagerURLConfigPrefix = "# ALERTMANAGER_URL "
)

type ManagerStore struct {
	rulesStoragePath string
}

func NewManagerStore(rulesStoragePath string) *ManagerStore {
	return &ManagerStore{rulesStoragePath: rulesStoragePath}
}

func (m *ManagerStore) Create(managerId, alertmanagerAddr string) (string, error) {
	managerFilePath := m.rulesFilePath(managerId)
	exists, err := m.rulesManagerExists(managerId)
	if err != nil {
		return managerFilePath, err
	}
	if exists {
		return managerFilePath, ManagerExistsError
	}

	err = m.write(managerId, nil, alertmanagerAddr)
	if err != nil {
		return managerFilePath, err
	}

	return managerFilePath, nil
}

func (m *ManagerStore) Load(managerId string) (string, string, error) {
	managerFilePath := m.rulesFilePath(managerId)
	alertmanagerAddr, err := extractAlertmanagerAddr(managerFilePath)
	if err != nil {
		return managerFilePath, alertmanagerAddr, err
	}

	return managerFilePath, alertmanagerAddr, nil
}

func (m *ManagerStore) AddRuleGroup(managerId string, ruleGroup *rulefmt.RuleGroup) error {
	exists, err := m.rulesManagerExists(managerId)
	if err != nil {
		return err
	}
	if !exists {
		return ManagerNotExistsError
	}

	managerFilePath := m.rulesFilePath(managerId)
	alertmanagerAddr, err := extractAlertmanagerAddr(managerFilePath)
	if err != nil {
		return err
	}

	err = m.write(managerId, ruleGroup, alertmanagerAddr)
	if err != nil {
		return err
	}

	return nil
}

func extractAlertmanagerAddr(ruleFile string) (string, error) {
	file, err := os.Open(ruleFile)
	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(file)

	var alertmanagerAddr string
	for scanner.Scan() {
		line := scanner.Text()

		matched := strings.HasPrefix(line, alertManagerURLConfigPrefix)
		if matched == true {
			alertmanagerAddr = strings.Replace(line, alertManagerURLConfigPrefix, "", 1)
			continue
		}
	}

	return alertmanagerAddr, nil
}

func (m *ManagerStore) write(managerId string, ruleGroup *rulefmt.RuleGroup, alertmanagerAddr string) error {
	managerFilePath := m.rulesFilePath(managerId)

	ruleGroups := rulefmt.RuleGroups{}
	if ruleGroup != nil {
		ruleGroups.Groups = []rulefmt.RuleGroup{*ruleGroup}
	}

	outBytes, err := yaml.Marshal(ruleGroups)
	if err != nil {
		return err
	}

	outBytes = append([]byte(alertManagerURLConfigPrefix+alertmanagerAddr+"\n"), outBytes...)

	err = ioutil.WriteFile(managerFilePath, outBytes, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func (m *ManagerStore) rulesManagerExists(managerId string) (bool, error) {
	managerFilePath := m.rulesFilePath(managerId)
	_, err := os.Stat(managerFilePath)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (m *ManagerStore) rulesFilePath(managerId string) string {
	return path.Join(m.rulesStoragePath, managerId)
}
