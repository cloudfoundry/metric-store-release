package rulesclient_test

import (
	. "github.com/cloudfoundry/metric-store-release/src/pkg/rulesclient"
	config_util "github.com/prometheus/common/config"
	prom_config "github.com/prometheus/prometheus/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ManagerConfig", func() {
	Describe("Validate()", func() {
		It("returns error when CACert is not valid", func() {
			alertManagers := &prom_config.AlertmanagerConfigs{
				{
					Scheme: "https",
					HTTPClientConfig: config_util.HTTPClientConfig{
						TLSConfig: config_util.TLSConfig{
							CAFile: "-----BEGIN foo-----GARBAGE-----END foo-----",
						},
					},
				},
			}

			managerConfig := NewManagerConfig("app-metrics", alertManagers)
			Expect(managerConfig.Validate()).ToNot(Succeed())
		})

		It("returns error when Cert or Key is not valid", func() {
			alertManagers := &prom_config.AlertmanagerConfigs{
				{
					Scheme: "https",
					HTTPClientConfig: config_util.HTTPClientConfig{
						TLSConfig: config_util.TLSConfig{
							CertFile: "-----BEGIN foo-----GARBAGE-----END foo-----",
						},
					},
				},
			}
			managerConfig := NewManagerConfig("app-metrics", alertManagers)
			Expect(managerConfig.Validate()).ToNot(Succeed())

			alertManagers = &prom_config.AlertmanagerConfigs{
				{
					Scheme: "https",
					HTTPClientConfig: config_util.HTTPClientConfig{
						TLSConfig: config_util.TLSConfig{
							KeyFile: "-----BEGIN foo-----GARBAGE-----END foo-----",
						},
					},
				},
			}
			managerConfig = NewManagerConfig("app-metrics", alertManagers)
			Expect(managerConfig.Validate()).ToNot(Succeed())
		})
	})
})
