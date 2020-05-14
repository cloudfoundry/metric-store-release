package testing

import "errors"

type ScrapeStoreSpy struct {
	Certs               map[string][]byte
	CaCerts             map[string][]byte
	PrivateKeys         map[string][]byte
	ScrapeConfig        []byte
	LoadedScrapeConfig  []byte
	NextSaveCertIsError bool
	NextSaveCAIsError   bool
	NextSaveKeyIsError  bool
}

func (spy *ScrapeStoreSpy) SaveCert(clusterName string, certData []byte) error {
	if spy.NextSaveCertIsError {
		return errors.New("Error Saving Certificate")
	}

	if spy.Certs == nil {
		spy.Certs = map[string][]byte{}
	}
	spy.Certs[clusterName] = certData
	return nil
}
func (spy *ScrapeStoreSpy) SaveCA(clusterName string, certData []byte) error {
	if spy.NextSaveCAIsError {
		return errors.New("Error Saving CA")
	}
	if spy.CaCerts == nil {
		spy.CaCerts = map[string][]byte{}
	}
	spy.CaCerts[clusterName] = certData
	return nil
}
func (spy *ScrapeStoreSpy) SavePrivateKey(clusterName string, keyData []byte) error {
	if spy.NextSaveKeyIsError {
		return errors.New("Error Saving Private Key")
	}
	if spy.PrivateKeys == nil {
		spy.PrivateKeys = map[string][]byte{}
	}
	spy.PrivateKeys[clusterName] = keyData
	return nil
}

func (spy *ScrapeStoreSpy) Path() string {
	return ""
}
func (spy *ScrapeStoreSpy) PrivateKeyPath(string) string {
	return "/tmp/scraper/private.key"
}
func (spy *ScrapeStoreSpy) CAPath(clusterName string) string {
	return "/tmp/scraper/" + clusterName + "/ca.pem"
}
func (spy *ScrapeStoreSpy) CertPath(clusterName string) string {
	return "/tmp/scraper/" + clusterName + "/cert.pem"
}

func (spy *ScrapeStoreSpy) SaveScrapeConfig(config []byte) error {
	spy.ScrapeConfig = config
	return nil
}
func (spy *ScrapeStoreSpy) LoadScrapeConfig() ([]byte, error) {
	return spy.LoadedScrapeConfig, nil
}
