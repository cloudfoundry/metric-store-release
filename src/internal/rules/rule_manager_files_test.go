package rules_test

import (
	"io/ioutil"
	"path/filepath"

	. "github.com/cloudfoundry/metric-store-release/src/internal/rules"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	config_util "github.com/prometheus/common/config"
	prom_config "github.com/prometheus/prometheus/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RuleManagerFile", func() {
	Describe("Create", func() {
		It("Writes an alertmanager config to the filesystem", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()
			caFile := testing.Cert("metric-store-ca.crt")
			certFile := testing.Cert("metric-store.crt")
			keyFile := testing.Cert("metric-store.key")

			ruleManagerFiles := NewRuleManagerFiles(tempStorage.Path())

			alertManagers := &prom_config.AlertmanagerConfigs{{
				Scheme: "https",
				HTTPClientConfig: config_util.HTTPClientConfig{
					TLSConfig: config_util.TLSConfig{
						CAFile:   caFile,
						CertFile: certFile,
						KeyFile:  keyFile,
					},
				},
			}}
			_, err := ruleManagerFiles.Create("app-metrics", alertManagers)
			Expect(err).NotTo(HaveOccurred())

			alertManager := []*prom_config.AlertmanagerConfig(*alertManagers)[0]
			Expect(alertManager.HTTPClientConfig.TLSConfig.CAFile).To(Equal(caFile))
			Expect(alertManager.HTTPClientConfig.TLSConfig.CertFile).To(Equal(certFile))
			Expect(alertManager.HTTPClientConfig.TLSConfig.KeyFile).To(Equal(keyFile))
		})

		Context("when configured with an inline cert", func() {
			It("extracts the cert to a file, and interpolates the file name", func() {
				tempStorage := testing.NewTempStorage()
				defer tempStorage.Cleanup()
				caCert := testing.MustAsset("metric-store-ca.crt")
				cert := testing.MustAsset("metric-store.crt")
				key := testing.MustAsset("metric-store.key")

				ruleManagerFiles := NewRuleManagerFiles(tempStorage.Path())

				alertManagers := &prom_config.AlertmanagerConfigs{
					{
						Scheme: "https",
						HTTPClientConfig: config_util.HTTPClientConfig{
							TLSConfig: config_util.TLSConfig{
								CAFile:   string(caCert),
								CertFile: string(cert),
								KeyFile:  string(key),
							},
						},
					},
					{
						Scheme: "https",
						HTTPClientConfig: config_util.HTTPClientConfig{
							TLSConfig: config_util.TLSConfig{
								CAFile:   string(caCert),
								CertFile: string(cert),
								KeyFile:  string(key),
							},
						},
					},
				}
				_, err := ruleManagerFiles.Create("app-metrics", alertManagers)
				Expect(err).NotTo(HaveOccurred())

				ruleManagerFileNames := tempStorage.Directory("app-metrics")
				Expect(ruleManagerFileNames).To(ContainElement("alertmanager-config-0-ca.crt"))
				Expect(ruleManagerFileNames).To(ContainElement("alertmanager-config-0.crt"))
				Expect(ruleManagerFileNames).To(ContainElement("alertmanager-config-0.key"))
				Expect(ruleManagerFileNames).To(ContainElement("alertmanager-config-1-ca.crt"))
				Expect(ruleManagerFileNames).To(ContainElement("alertmanager-config-1.crt"))
				Expect(ruleManagerFileNames).To(ContainElement("alertmanager-config-1.key"))

				caCertFilePath := filepath.Join(tempStorage.Path(), "app-metrics", "alertmanager-config-0-ca.crt")
				data, err := ioutil.ReadFile(caCertFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(Equal(caCert))

				certFilePath := filepath.Join(tempStorage.Path(), "app-metrics", "alertmanager-config-0.crt")
				data, err = ioutil.ReadFile(certFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(Equal(cert))

				keyFilePath := filepath.Join(tempStorage.Path(), "app-metrics", "alertmanager-config-0.key")
				data, err = ioutil.ReadFile(keyFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(Equal(key))

				alertManager := []*prom_config.AlertmanagerConfig(*alertManagers)[0]
				Expect(alertManager.HTTPClientConfig.TLSConfig.CAFile).To(Equal(caCertFilePath))
				Expect(alertManager.HTTPClientConfig.TLSConfig.CertFile).To(Equal(certFilePath))
				Expect(alertManager.HTTPClientConfig.TLSConfig.KeyFile).To(Equal(keyFilePath))

				alertmanagerConfigFilePath := filepath.Join(tempStorage.Path(), "app-metrics", "alertmanager.yml")
				data, err = ioutil.ReadFile(alertmanagerConfigFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).NotTo(ContainSubstring("-----BEGIN"))
			})
		})
	})

	Describe("Delete", func() {
		It("deletes the directory for a manager", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			ruleManagerFiles := NewRuleManagerFiles(tempStorage.Path())

			_, err := ruleManagerFiles.Create("app-metrics", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(tempStorage.Directories()).To(ConsistOf("app-metrics"))

			Expect(ruleManagerFiles.Delete("app-metrics")).To(Succeed())
			Expect(tempStorage.Directories()).To(BeEmpty())
		})

		It("errors if manager file doesn't exist", func() {
			tempStorage := testing.NewTempStorage()
			defer tempStorage.Cleanup()

			ruleManagerFiles := NewRuleManagerFiles(tempStorage.Path())

			Expect(tempStorage.Directories()).To(BeEmpty())

			err := ruleManagerFiles.Delete("app-metrics")
			Expect(err).To(MatchError(ManagerNotExistsError))
		})
	})
})
