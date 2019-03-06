package tls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"log"

	"google.golang.org/grpc/credentials"
)

type TLS struct {
	CAPath   string `env:"CA_PATH,   required, report"`
	CertPath string `env:"CERT_PATH, required, report"`
	KeyPath  string `env:"KEY_PATH,  required, report"`
}

func (t TLS) Credentials(cn string) credentials.TransportCredentials {
	creds, err := NewTLSCredentials(t.CAPath, t.CertPath, t.KeyPath, cn)
	if err != nil {
		log.Fatalf("failed to load TLS config: %s", err)
	}

	return creds
}

func NewTLSCredentials(
	caPath string,
	certPath string,
	keyPath string,
	cn string,
) (credentials.TransportCredentials, error) {
	cfg, err := NewMutualTLSConfig(caPath, certPath, keyPath, cn)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(cfg), nil
}

func NewMutualTLSConfig(caPath, certPath, keyPath, cn string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	tlsConfig := NewBaseTLSConfig()
	tlsConfig.ServerName = cn
	tlsConfig.Certificates = []tls.Certificate{cert}

	caCertBytes, err := ioutil.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCertBytes); !ok {
		return nil, errors.New("cannot parse ca cert")
	}

	tlsConfig.RootCAs = caCertPool

	return tlsConfig, nil
}

func NewBaseTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		CipherSuites:       supportedCipherSuites,
	}
}

var supportedCipherSuites = []uint16{
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}
