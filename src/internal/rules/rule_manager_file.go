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
	alertManagerURLConfigPrefix = "# ALERTMANAGER_URL "
)

type RuleManagerFile struct {
	rulesStoragePath string
}

func NewRuleManagerFile(rulesStoragePath string) *RuleManagerFile {
	return &RuleManagerFile{rulesStoragePath: rulesStoragePath}
}

func (f *RuleManagerFile) Create(managerId, alertmanagerAddr string) (string, error) {
	managerFilePath := f.rulesFilePath(managerId)
	exists, err := f.rulesManagerExists(managerId)
	if err != nil {
		return managerFilePath, err
	}
	if exists {
		return managerFilePath, ManagerExistsError
	}

	err = f.write(managerId, nil, alertmanagerAddr)
	return managerFilePath, err
}

func (f *RuleManagerFile) DeleteManager(managerId string) error {
	managerFilePath := f.rulesFilePath(managerId)
	exists, err := f.rulesManagerExists(managerId)
	if err != nil {
		return err
	}
	if !exists {
		return ManagerNotExistsError
	}

	return f.remove(managerFilePath)
}

func (f *RuleManagerFile) Load(managerId string) (string, string, error) {
	managerFilePath := f.rulesFilePath(managerId)
	alertmanagerAddr, err := extractAlertmanagerAddr(managerFilePath)
	if err != nil {
		return managerFilePath, alertmanagerAddr, err
	}

	return managerFilePath, alertmanagerAddr, nil
}

func (f *RuleManagerFile) UpsertRuleGroup(managerId string, ruleGroup *rulefmt.RuleGroup) error {
	exists, err := f.rulesManagerExists(managerId)
	if err != nil {
		return err
	}
	if !exists {
		return ManagerNotExistsError
	}

	managerFilePath := f.rulesFilePath(managerId)
	alertmanagerAddr, err := extractAlertmanagerAddr(managerFilePath)
	if err != nil {
		return err
	}

	return f.write(managerId, ruleGroup, alertmanagerAddr)
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

func (f *RuleManagerFile) write(managerId string, ruleGroup *rulefmt.RuleGroup, alertmanagerAddr string) error {
	managerFilePath := f.rulesFilePath(managerId)

	ruleGroups := rulefmt.RuleGroups{}
	if ruleGroup != nil {
		ruleGroups.Groups = []rulefmt.RuleGroup{*ruleGroup}
	}

	outBytes, err := yaml.Marshal(ruleGroups)
	if err != nil {
		return err
	}

	outBytes = append([]byte(alertManagerURLConfigPrefix+alertmanagerAddr+"\n"), outBytes...)

	return ioutil.WriteFile(managerFilePath, outBytes, os.ModePerm)
}

func (f *RuleManagerFile) remove(filePath string) error {
	return os.Remove(filePath)
}

func (f *RuleManagerFile) rulesManagerExists(managerId string) (bool, error) {
	managerFilePath := f.rulesFilePath(managerId)
	_, err := os.Stat(managerFilePath)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (f *RuleManagerFile) rulesFilePath(managerId string) string {
	return path.Join(f.rulesStoragePath, managerId)
}
