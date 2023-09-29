package testing

//go:generate rm -rf certs/
//go:generate ../../../scripts/generate-certs
//go:generate go-bindata -nocompress -pkg testing -o bindata.go -prefix certs/ certs/

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
// > openssl x509 -req -in localhost.csr -CA metric-store-ca.crt -CAkey metric-store-ca.key -CAcreateserial -out localhost.crt -days 3652 -sha256 -extfile ../ssl.cnf
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
MIIFGjCCAwKgAwIBAgIJANCFTxfmfZSQMA0GCSqGSIb3DQEBCwUAMBoxGDAWBgNV
BAMTD21ldHJpYy1zdG9yZS1jYTAeFw0yMzA5MjkxMjMyMjJaFw0zMzA5MjgxMjMy
MjJaMIGHMQswCQYDVQQGEwJBQTEQMA4GA1UECAwHWWVyZXZhbjEPMA0GA1UEBwwG
UW9jaGFyMQ8wDQYDVQQKDAZWTXdhcmUxEDAOBgNVBAsMB1Bpdm90YWwxDTALBgNV
BAMMBHRlc3QxIzAhBgkqhkiG9w0BCQEWFGhtYW51a3lhbkB2bXdhcmUuY29tMIIB
IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEApE91Uh3WaUoBPEiSfKrkpR+T
/9wkGw2eo7vfWX6ECkpAXMX7om6WQ1MiU8rOruMRVtwfNN9XjWghVeG7BJt8a7i1
pXkbJ37Qf7P4Xgkpf/yKpUXJgS9KTlFIoABJ7BRWr1zYSssQUSYGDjPN4skbaQ15
TDDNqzTTrMxNxrDzPZpoMr2wSROHlOMaJz+1lfkQiU41WT4tOyj79sV2/AEu8sqW
AJ1Jfsvrn3kw84UjqlI90qQI+52Ub9FlgCRxRd1MQK+T9D8Iu5MgvCgswPDEIGmX
1CiQswetZC9OlhzWb/uZ/4G+a/+Hs4Ue2E+qrTezYX0cN/Lx7KFWklyLP+73cwID
AQABo4H0MIHxMAkGA1UdEwQCMAAwEQYJYIZIAYb4QgEBBAQDAgZAMDMGCWCGSAGG
+EIBDQQmFiRPcGVuU1NMIEdlbmVyYXRlZCBTZXJ2ZXIgQ2VydGlmaWNhdGUwHQYD
VR0OBBYEFLXvuV9NrnKYsz2GV8aRHj8/TdaqMEIGA1UdIwQ7MDmAFF7vtw5ffhkt
mUzXnb7T2Ctsy5Z+oR6kHDAaMRgwFgYDVQQDEw9tZXRyaWMtc3RvcmUtY2GCAQEw
DgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQMMAoGCCsGAQUFBwMBMBQGA1UdEQQNMAuC
CWxvY2FsaG9zdDANBgkqhkiG9w0BAQsFAAOCAgEAn24VZ0tzlOTD9WzFlkUEotNE
vbhPkRXi/qX+ZMA3b8nPK2flTgkiobgA5csvMLqgavFjiifNEDnHbU9SES2F4NrB
IzogcfrtuF46vUiBsA3QHsDu3CuFUlWZvT042cjvIS1DM3Cq0tRu4ubZWfMz9R73
d+NacWRgneQxtw2I1Tu6IOc7XmYAWwdrjS7HzHpXirPKGI5gcnHk46+BtnUndx9G
d9kxVlPfUzePO2K9qlyE6uCDHsLxxFFkP7UDZTXIiIaVkVbrAskNHBf+e1P+iKJF
Ukavvj1TGLez92twmh+CfxRY+i9D3OrxmSyOq25UkIs3lco8/lEQp0/j9QRKHxFw
N8SJlz53wTE893uBiblbYPatx2G7RzcWAKTb+9mQDS2CLEDIDJppVrbKD57doGPy
8t/l71NUcVan5p6uLLcVoqLCLRgubD8RMWAMLGZW2iRDH6k+TwOXJF+juKrCG99U
xQmRGCv8uu29j/a3jbWfg0yZaGGFeBrQ0H2VH6v4zuPraHWiWmr1GBtrfCPxmzeE
WSAvPm/89SA0oSUO8Ek8dtQsQed2FeK/tlrG9fg2NTKTdUOqj2Mx8aM9AvKiSM6G
WGFilB0uXGOPk+nfMvyYi2uSbBk3qo0x2nfG7N+HgeP7lHNh4+oVGu+F3sd+hQ2M
pKVCanVS4qbmatd/r0w=
-----END CERTIFICATE-----`)

var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEApE91Uh3WaUoBPEiSfKrkpR+T/9wkGw2eo7vfWX6ECkpAXMX7
om6WQ1MiU8rOruMRVtwfNN9XjWghVeG7BJt8a7i1pXkbJ37Qf7P4Xgkpf/yKpUXJ
gS9KTlFIoABJ7BRWr1zYSssQUSYGDjPN4skbaQ15TDDNqzTTrMxNxrDzPZpoMr2w
SROHlOMaJz+1lfkQiU41WT4tOyj79sV2/AEu8sqWAJ1Jfsvrn3kw84UjqlI90qQI
+52Ub9FlgCRxRd1MQK+T9D8Iu5MgvCgswPDEIGmX1CiQswetZC9OlhzWb/uZ/4G+
a/+Hs4Ue2E+qrTezYX0cN/Lx7KFWklyLP+73cwIDAQABAoIBADT/mDUTgLxXbYyX
UAC8UqHcTK2jwVWkj/36NHG2eDqldps2FRNwnjE7GIB0kzQf89DTkZFJVQE8jDwa
Ymt7B6frXVPxe0vDANegIkWaqPMqb0Id/4AW093tJALP9QgcC1XnGbEOTMYQALYG
CavT+G5rNSvZNg26LWi4QYIMQ3kj14YSGJBqATDv2gKS7lU9hGXQvZWULJKurgpn
ptui/NkR3lFWjK8aa6aLPnAPp3+KisTyJxGYBoL7OPQHEy2P5es/Uw9W6JEoLaoi
UXFsWH6ZP/2yeC4Q/wX7zhYkSDE8d6kY+JUC/x0Tg4ffNvVPwGFbJJlOJYge/vWV
l8B1YUECgYEA077rfJeK4jQYv8UyFY8syLBoyMriNAEmU3C6STc62+ytjI+XM/tX
eKEfAtyUJeiZ7DBH1z7cfSdhNlSDNlf4GnjdQQIlj+ga4UFVo7iuBqRyBkrjE0nr
Xr3zt0f8LMBEWtzjffE/YJ1mA2chqYQDudsiIofz5FsNbmLqMcARneECgYEAxqaX
l6hxs6UOR8z2vNvw/cz9dUu+S51ljfPaBRMR3NOUnAW0KFAbGi1LCthLKGtDkliJ
XZeDjqFLeorBibxKXs4RSz9jT0lwfMdGaXOiIlby7FDYCc+lgQEZO7QRWfIe7J7s
RToZRV+ZbRjpIp2Jysow55bYpBvY02i74R9Ot9MCgYEAqAMIVR7li2Ds9lUgzWyC
i2c2bYRWAg+ben+qwGd7Y3+joTFaN1vKZyPpOFsPjhjG8VrJ1ifByeiSQQrD5j3c
1hxq6qcqaMoxceRmcHccVpbrBsUq8mYnxVARbq9Gj7erRTGZrJfcwuuBQ1f0pM3k
KveOWTnospvwx1LjIsCU+eECgYBf/6vsj2t7LD2tdyCZ/hQFIuYtpA/vTL4CDqEC
qMeOFvWPPLZmcOfYC6FjOUmA2+1IsN6ZSxo5eDsYmiuTW1n5XM5Atf5RF6Vzt32Q
gmANBkXY6+yrORy7LgO6tXdZJ0fIg7icb8o8m0lyzoIDx2wKgxGFKYHCNO7go5F/
5nhNHwKBgEDz8M+OzXYg0y9qBNTaYbwEpOzHk3FL4Lei/qct8k/gzN5n/FshP6/W
jBQWqa1w5CqCwcqmAN6dDwRO5SJWIv3UeX+wyAear440ebfFuGNw5Fuf1m7lqJth
9A7NlLz+9jfzaPsOXeTiCvN1pDw+sNvHepkII2++pRPN7e1T7Wuo
-----END RSA PRIVATE KEY-----`)
