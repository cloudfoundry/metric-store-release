package store_test

import (
	store2 "github.com/cloudfoundry/metric-store-release/src/internal/cluster-discovery/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"
	"path/filepath"
)

var _ = Describe("Scrape Storage Manager", func() {
	Describe("private key creation", func() {
		type testContext struct {
			store   *store2.ScrapeStore
			rootDir string
		}
		var setup = func() (*testContext, func()) {
			rootDir, err := ioutil.TempDir("", "")
			Expect(err).ToNot(HaveOccurred())

			scrapeFile, err := ioutil.TempFile("", "")
			Expect(err).ToNot(HaveOccurred())
			defer scrapeFile.Close()

			store, err := store2.LoadCertStore(rootDir, nil)
			Expect(err).ToNot(HaveOccurred())

			tc := &testContext{
				store:   store,
				rootDir: rootDir,
			}
			return tc,
				func() {
					os.RemoveAll(tc.rootDir)
				}
		}

		XIt("creates a private key when one doesn't exist", func() {
			//tc, teardown := setup()
			//defer teardown()
			//
			//Expect(tc.store.PrivateKeyPath("")).To(Equal(filepath.Join(tc.store.Path(), "private.key")))
			//Expect(tc.store.PrivateKey()).ToNot(Equal([]byte{}))
		})

		It("stores CA per cluster", func() {
			tc, teardown := setup()
			defer teardown()

			err := tc.store.SaveCA("clusterName", []byte("cadata"))
			Expect(err).ToNot(HaveOccurred())
			caFileName := tc.store.CAPath("clusterName")
			Expect(caFileName).To(ContainSubstring("ca.pem"))

			fileContents, err := ioutil.ReadFile(caFileName)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileContents).To(Equal([]byte("cadata")))
		})

		It("stores cluster cert", func() {
			tc, teardown := setup()
			defer teardown()

			err := tc.store.SaveCert("clusterName", []byte("cert"))
			Expect(err).ToNot(HaveOccurred())
			certPath := tc.store.CertPath("clusterName")
			Expect(certPath).To(ContainSubstring("cert.pem"))

			fileContents, err := ioutil.ReadFile(certPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileContents).To(Equal([]byte("cert")))
		})

		It("stores cluster private key", func() {
			tc, teardown := setup()
			defer teardown()

			err := tc.store.SavePrivateKey("clusterName", []byte("private key"))
			Expect(err).ToNot(HaveOccurred())
			keyPath := tc.store.PrivateKeyPath("clusterName")
			Expect(keyPath).To(ContainSubstring("private.key"))

			fileContents, err := ioutil.ReadFile(keyPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileContents).To(Equal([]byte("private key")))
		})

		It("Store scrape config", func() {
			tc, teardown := setup()
			defer teardown()

			err := tc.store.SaveScrapeConfig([]byte("config"))
			Expect(err).ToNot(HaveOccurred())

			fileContents, err := ioutil.ReadFile(filepath.Join(tc.rootDir, "scrape_config.yml"))
			Expect(err).ToNot(HaveOccurred())
			Expect(fileContents).To(Equal([]byte("config")))
		})

		XIt("deletes store when recreating private key", func() {
			tc, teardown := setup()
			defer teardown()

			privateKeyFileName := tc.store.PrivateKeyPath("")
			err := tc.store.SaveCA("clusterName", []byte("cacert"))
			Expect(err).ToNot(HaveOccurred())

			os.Remove(privateKeyFileName)

			_, err = store2.LoadCertStore(tc.store.Path(), nil)
			Expect(err).ToNot(HaveOccurred())

			_, err = os.Open(tc.store.CAPath("clusterName"))
			Expect(err).To(HaveOccurred())
		})

		It("returns a scrape config", func() {
			tc, teardown := setup()
			defer teardown()

			err := ioutil.WriteFile(filepath.Join(tc.rootDir, "scrape_config.yml"), []byte("config"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			fileContents, err := tc.store.LoadScrapeConfig()
			Expect(err).ToNot(HaveOccurred())
			Expect(fileContents).To(Equal([]byte("config")))
		})
	})
})
