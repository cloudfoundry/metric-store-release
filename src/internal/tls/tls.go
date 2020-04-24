package tls

import (
	"crypto/tls"

	"code.cloudfoundry.org/tlsconfig"
)

type TLS struct {
	CAPath   string `env:"CA_PATH,   required, report"`
	CertPath string `env:"CERT_PATH, required, report"`
	KeyPath  string `env:"KEY_PATH,  required, report"`
}

func NewMutualTLSClientConfig(caPath, certPath, keyPath, commonName string) (*tls.Config, error) {
	return tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certPath, keyPath),
	).Client(
		tlsconfig.WithAuthorityFromFile(caPath),
		tlsconfig.WithServerName(commonName),
	)
}

func NewMutualTLSServerConfig(caPath, certPath, keyPath string) (*tls.Config, error) {
	return tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certPath, keyPath),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(caPath),
	)
}

func NewTLSClientConfig(caPath, commonName string) (*tls.Config, error) {
	return tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
	).Client(
		tlsconfig.WithAuthorityFromFile(caPath),
		tlsconfig.WithServerName(commonName),
	)
}

func NewTLSServerConfig(certPath, keyPath string) (*tls.Config, error) {
	return tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certPath, keyPath),
	).Server()
}

func NewGenericTLSConfig() (*tls.Config, error) {
	return tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
	).Server()
}

func NewUAATLSConfig(caPath string, skipCertVerify bool) (*tls.Config, error) {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
	).Client(
		tlsconfig.WithAuthorityFromFile(caPath),
	)
	tlsConfig.InsecureSkipVerify = skipCertVerify

	return tlsConfig, err
}

func NewCAPITLSConfig(caPath string, skipCertVerify bool, commonName string) (*tls.Config, error) {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
	).Client(
		tlsconfig.WithAuthorityFromFile(caPath),
		tlsconfig.WithServerName(commonName),
	)
	tlsConfig.InsecureSkipVerify = skipCertVerify

	return tlsConfig, err
}
