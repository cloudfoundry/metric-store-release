package testing

//go:generate ../../../scripts/generate-certs
//go:generate go-bindata -nocompress -pkg testing -o bindata.go -prefix certs/ certs/
//go:generate rm -rf certs/

import (
	"crypto/tls"
	"io/ioutil"
	"log"

	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
)

func Cert(filename string) string {
	var contents []byte
	switch filename {
	case "localhost.crt":
		contents = localhostCert
	case "localhost.key":
		contents = localhostKey
	default:
		contents = MustAsset(filename)
	}

	tmpfile, err := ioutil.TempFile("", "")

	if err != nil {
		log.Fatal(err)
	}

	if _, err := tmpfile.Write(contents); err != nil {
		log.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}

	return tmpfile.Name()
}

// NOTE: these certs are signed by the Metric Store CA. here are the steps to regenerate:
//
// 1. create a new private key
// > openssl genrsa -out localhost.key 2048
//
// 2. create a certificate signing request for that key (use `localhost` for the common name)
// > openssl req -new -key localhost.key -out localhost.csr
//
// 3. use the MS CA to create a cert from that signing request
// > openssl x509 -req -in localhost.csr -CA ms_ca.crt -CAkey ms_ca.key -CAcreateserial -out localhost.crt -days 3652 -sha256
//
// 4. copy the contents of localhost.crt and localhost.key into the variables below
//
// The Metric Store CA cert & key are stored in bindata.go

func MutualTLSServerConfig() *tls.Config {
	tlsConfig, err := sharedtls.NewMutualTLSServerConfig(
		Cert("metric-store-ca.crt"),
		Cert("metric-store.crt"),
		Cert("metric-store.key"),
	)

	if err != nil {
		panic("could not create MutualTLSServerConfig")
	}

	return tlsConfig
}

func MutualTLSClientConfig() *tls.Config {
	tlsConfig, err := sharedtls.NewMutualTLSClientConfig(
		Cert("metric-store-ca.crt"),
		Cert("metric-store.crt"),
		Cert("metric-store.key"),
		"metric-store",
	)

	if err != nil {
		panic("could not create MutualTLSClientConfig")
	}

	return tlsConfig
}

var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIEJTCCAg0CCQDTRI6ICVP+YDANBgkqhkiG9w0BAQsFADAaMRgwFgYDVQQDEw9t
ZXRyaWMtc3RvcmUtY2EwHhcNMjAwMzI0MjAxMDU3WhcNNDcwODA5MjAxMDU3WjCB
jjELMAkGA1UEBhMCVVMxETAPBgNVBAgMCENvbG9yYWRvMQ8wDQYDVQQHDAZEZW52
ZXIxDzANBgNVBAoMBlZNd2FyZTELMAkGA1UECwwCQ0YxEjAQBgNVBAMMCWxvY2Fs
aG9zdDEpMCcGCSqGSIb3DQEJARYaY2YtbWV0cmljLXN0b3JlQHBpdm90YWwuaW8w
ggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDqvEVW5woIPeJYISufRj3d
3Y7OdhSY4JexZiQHyG5rAIYJb3l6FVHpeRm9ImD1JqVhD/DQcRCxVu2GEzOR9bbQ
XLkVlobSWsIsZ+k3GC4pSHxHSJPtlUnKG4IAitdZ4P4Yfw6gxCIA7cUHG6YsLX6L
sjLTRagdZFo+52hnzgGQWMnphdKJ5M+FfH4/M4u+oN0v1U0swFdIGibmPvkBPsQY
WGlDkzX9vSiXkI4CcoxmHCkLCZh0O8P95Gb5rjYPKd7mct0tGgJ6zHJktop1rE0l
0sdJqquNDRqox5OdpK7OzKJ9duh5hX0Q7aP23W7WzzRbMbAoK2Bv7h8hz50RQ1jv
AgMBAAEwDQYJKoZIhvcNAQELBQADggIBAIGJAqAxnJNZAnpLmEln/D9Dofg4J4AN
zwyu+t0kS6HaAHixlM+wdUlIj7l5TvdMUYZpkELPRdlRn9r19fV5W9IInhOU4+Gy
K/3Mru9Delb/SdHmE/vSrFMsXpbuSDafVt+3S6eqvouiBTRMKzN4widohnp8X05I
pQjhbcpa7ZzLtnc4ikKJ209UtO0SbVfLuP8YkaGpQbHz0zQ4+qf7PT+Vh2aR2VNj
gtMvltYzESbLfHpuW4f62kkqJ8Zq8/C/vGPTFLuolGcUiBjyN392qIMssml3PTnE
6NkbguhRGE+W71ydGs3j9lkNoCjlm1fBU02mYoK1vXVcq2U7c9j3K+MeOV1y5gzq
VwW3/bocqkomYbrzVJtlc2HYal9JjkdTbOHsE5zfiaFJHbP/3C4lPgLJiPQPHmoF
kkEYcKspywonyeYDXxHILbse4pazLMy1QmBDF5E5b2SN5uWSfjSz+5FC5UUQHSas
UdEpVMP1n1Tgwo/wTFc+jar6AlpbYPp09MBOqPe7uqNaS5Hgq+q1oI4WlXZQM/fp
r+seAu5YolrhUOZ99uxxBoE/ef45OUVzCJ4OKm8C6FYFtAlZoMUtPUyzHmXYg1FE
ViHkXYE3pzXA/SN4f/Ei+eEFXg21PRFs/ppTZ5XZS4UVK/8lh7DO5YDLQYylFl/F
AzR4GSAjNUD4
-----END CERTIFICATE-----`)

var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA6rxFVucKCD3iWCErn0Y93d2OznYUmOCXsWYkB8huawCGCW95
ehVR6XkZvSJg9SalYQ/w0HEQsVbthhMzkfW20Fy5FZaG0lrCLGfpNxguKUh8R0iT
7ZVJyhuCAIrXWeD+GH8OoMQiAO3FBxumLC1+i7Iy00WoHWRaPudoZ84BkFjJ6YXS
ieTPhXx+PzOLvqDdL9VNLMBXSBom5j75AT7EGFhpQ5M1/b0ol5COAnKMZhwpCwmY
dDvD/eRm+a42Dyne5nLdLRoCesxyZLaKdaxNJdLHSaqrjQ0aqMeTnaSuzsyifXbo
eYV9EO2j9t1u1s80WzGwKCtgb+4fIc+dEUNY7wIDAQABAoIBAF4yHPUpk5oJE1pg
PTwWGN9+eD8bnVpXzievIEhLQxwHQsJojGvUQGGbahu+vv/BeV4A4pcSuCsiAgDq
lag93RWyD8e89u9U4lSlgi3Ms0F3x/9m/Y26ebjz3vBOxupXYj/8RKd47VhIEeev
TbiurPhsEv87FaJt0dUqUXhOb8w3QFhJM32br7kxdCQdbBE4z7Q7TMtCFjc68EnJ
AUSedaMXsf0vcjj9zv6R7seYAozM1SEmen6Cw20akMAuKEhJnIw7XEFjEA/saUI4
0JClRx+0JJNKDf9bxcgmCudh+hsJU3g8ra07CowEpBuarFuU6TtlVoxvfseLWDeq
V7G/bAkCgYEA9pTAUfAW7O53Tg8MAcW8b/xvyIWyAu1q0ikJsXluRvcN5iny0DkO
bYLQa1isyfhU+4GJz602bBB4J/cqpaJRRCH2ZziMwoi0s9coQrRNqXGu8jbJjHuf
jV17O7rnZAyw0Fufm91UxN9dhPvpzZPxvGlPDV2vqB8MsHVj3XeMqX0CgYEA87Ov
ORDhkICjvF1sO0PO1jkF3B5mB7a16oCrEBaPPOX6L0Kw4iDYgpl3q6hGhbBvJu/+
legTY/rMN5XIn7aabwsAbrSsOrpwCqYa0K6YC1qNre2j0AC3bueaJflpW+Ft3ZV4
5dFXgzMyTS0sL6GbBk4BGsHvq/t4r07/CIS+t9sCgYBfMrLFb6IKO07ITjrefE7y
FU265xMA2lSBauKZKD6RG1S8Zbme9khBs11v9D5Rg5SbvTlNepwmQH2DQIOwiuhB
G7ObylNdz5WkUQ70IdRR9NgMH2bU2+2PkGXBe7lWAShKaPVIIb1WfL4IV5G+kr2j
dizVBjSI/ePSRKAXos4lqQKBgQDAmgrqy+epP7GoBiGquQ14CvsRm0jB4enmGqiX
f2zXEV7oCQoovRLALLACj2yk7er62APZz9+7TZQmfg9gAn4NMqG13L6db4lrMRnS
QZpSps+AXWbw1hAi65HNX0+gWQpubFpvL0K9ozGnAwN/5XXSxsVis1FLF+SqkIFI
5zifrwKBgQDDCoJwQWRBNITsU2cqtan14eHE876TGuggZQk+PNpCopZQGmbHeZOy
IsUhmgOh3Zue4yr7ukyGYbF41WwJLDDlvlp7ygrmPVsB2UtPIohw/Vp9RMNrsiqr
pglqJy94kcNtTO8cuuqde2x5bRJUSvJnx3FlASFXPgA8u3Zs+/N7VQ==
-----END RSA PRIVATE KEY-----`)
