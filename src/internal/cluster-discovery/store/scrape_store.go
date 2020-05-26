package store

import (
	"bytes"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"k8s.io/client-go/util/keyutil"
	prometheusConfig "github.com/prometheus/prometheus/config"
	"os"
	"path/filepath"
)

type ScrapeStore struct { // TODO: rename to ScrapeConfigStore // TODO but also certs
	rootDir string
	log     *logger.Logger
}

func LoadCertStore(storeDir string, log *logger.Logger) (*ScrapeStore, error) {
	err := ensureStoreConsistency(storeDir, log)
	if err != nil {
		return nil, err
	}

	return &ScrapeStore{
		rootDir: storeDir,
		log:     log,
	}, nil
}

func (store *ScrapeStore) CAPath(clusterName string) string {
	return filepath.Join(store.Path(), clusterName, "ca.pem")
}

func (store *ScrapeStore) CertPath(clusterName string) string {
	return filepath.Join(store.Path(), clusterName, "cert.pem")
}

func (store *ScrapeStore) PrivateKeyPath(clusterName string) string {
	return filepath.Join(store.Path(), clusterName, "private.key")
}

func (store *ScrapeStore) Path() string {
	return store.rootDir
}

func (store *ScrapeStore) SaveCA(clusterName string, bytes []byte) error {
	return keyutil.WriteKey(store.CAPath(clusterName), bytes)
}

func (store *ScrapeStore) SaveCert(clusterName string, bytes []byte) error {
	return keyutil.WriteKey(store.CertPath(clusterName), bytes)
}

func (store *ScrapeStore) SavePrivateKey(clusterName string, bytes []byte) error {
	return keyutil.WriteKey(store.PrivateKeyPath(clusterName), bytes)
}

func (store *ScrapeStore) SaveScrapeConfig(bytes []byte) error {
	targetLocation := filepath.Join(store.rootDir, "scrape_config.yml")
	store.log.Debug("Writing config", logger.String("file", targetLocation), zap.ByteString("contents", bytes))
	return ioutil.WriteFile(targetLocation, bytes, os.ModePerm)
}

func (store *ScrapeStore) LoadScrapeConfig() ([]*prometheusConfig.ScrapeConfig, error) {
	file, err := ioutil.ReadFile(filepath.Join(store.rootDir, "scrape_config.yml"))
	if err != nil {
		return nil, err
	}

	var config prometheusConfig.Config
	err = yaml.NewDecoder(bytes.NewReader(file)).Decode(&config)
	if err != nil {
		return nil, err
	}
	return config.ScrapeConfigs, nil
}

// TODO rename or expand?
func ensureStoreConsistency(storeDir string, log *logger.Logger) error {
	log.Debug("Using scrape directory", logger.String("path", storeDir))
	if !directoryExists(storeDir, log) {
		err := os.Mkdir(storeDir, 0700)
		if err != nil {
			return err
		}
	}
	return nil
}

func directoryExists(path string, log *logger.Logger) bool {
	info, err := os.Stat(path)
	if err != nil {
		log.Error("scrape directory lookup", err)
	}

	if os.IsNotExist(err) {
		log.Debug("scrape directory stat error", zap.Bool("IsNotExist", true))
		return false
	}

	log.Debug("scrape directory type", zap.Bool("isDir", info.IsDir()))
	return info.IsDir()
}
