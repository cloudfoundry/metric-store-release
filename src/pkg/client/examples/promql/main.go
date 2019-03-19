package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	envstruct "code.cloudfoundry.org/go-envstruct"
	metricstore_client "github.com/cloudfoundry/metric-store-release/src/pkg/client"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <query>", os.Args[0])
	}

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	client := metricstore_client.NewClient(
		cfg.MetricStoreAddr,
		metricstore_client.WithViaGRPC(
			grpc.WithTransportCredentials(cfg.TLS.Credentials("metric-store")),
		),
	)

	result, err := client.PromQL(context.Background(), os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	m := &jsonpb.Marshaler{}
	str, err := m.MarshalToString(result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(str)
}

// Config is the configuration for a MetricStore Gateway.
type Config struct {
	MetricStoreAddr string `env:"METRIC_STORE_ADDR, required"`
	TLS             TLS
}

type TLS struct {
	CAPath   string `env:"CA_PATH,   required"`
	CertPath string `env:"CERT_PATH, required"`
	KeyPath  string `env:"KEY_PATH,  required"`
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
	cfg, err := NewTLSConfig(caPath, certPath, keyPath, cn)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(cfg), nil
}

func NewTLSConfig(caPath, certPath, keyPath, cn string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		ServerName:         cn,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: false,
	}

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

func LoadConfig() (*Config, error) {
	c := Config{}

	if err := envstruct.Load(&c); err != nil {
		return nil, err
	}

	return &c, nil
}
