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
MIIE8zCCAtugAwIBAgIJAKUmGKM5S+iXMA0GCSqGSIb3DQEBCwUAMBoxGDAWBgNV
BAMTD21ldHJpYy1zdG9yZS1jYTAeFw0yMzEwMDMwOTM3MjlaFw0zMzEwMDIwOTM3
MjlaMGExCzAJBgNVBAYTAkFBMQswCQYDVQQIDAJBQTELMAkGA1UEBwwCQUExCzAJ
BgNVBAoMAkFBMQswCQYDVQQLDAJBQTELMAkGA1UEAwwCQUExETAPBgkqhkiG9w0B
CQEWAkFBMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEArELt505QN90H
xj24ztPiHQ7PPdnG9wZbW2uKIDKj3JHmoyxKHI/yZHyfnokIwbbXA+kiAbUebB+2
D4C305QT+E+tHjTnsUpzP1bdcr3VuclzhjlVUWEyvpgXsLl4Ig6WDOIQtHK9nOz6
gIBzdG9twxJTSUcACMUv85zmn/qxjHYlF3sYABO1lIIQr3MRDVpBYooy0sJHHaSj
KBP48NlRil6ePiTFlL79DxE22Bsj0HabythPDFdIas8eNo5ONlM2EGdQpwfxf3jp
qv0rNtFiKdrGbEi4X26rWiJ7wZwnj8eEz7tjAMY+m800aUg3zzYfNlElTn0aDEg6
tU6nlu5vRwIDAQABo4H0MIHxMAkGA1UdEwQCMAAwEQYJYIZIAYb4QgEBBAQDAgZA
MDMGCWCGSAGG+EIBDQQmFiRPcGVuU1NMIEdlbmVyYXRlZCBTZXJ2ZXIgQ2VydGlm
aWNhdGUwHQYDVR0OBBYEFIzUPYScl44CyPjZwFZTu9vqq3O6MEIGA1UdIwQ7MDmA
FBIZdB0OJeM2hWRZSKk5+gNu+THpoR6kHDAaMRgwFgYDVQQDEw9tZXRyaWMtc3Rv
cmUtY2GCAQEwDgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQMMAoGCCsGAQUFBwMBMBQG
A1UdEQQNMAuCCWxvY2FsaG9zdDANBgkqhkiG9w0BAQsFAAOCAgEAUaVUQ/eCduso
+6otbdaVU9LIrV0nNMRP2L8Al2HV2VYCSYeTyl9U4RtMdiCYcD0U4psC4RF7DyGu
qzk6gifx11drbLjKZO6MsbQvOBPbw4cz9zB6R557v7F13bKjeeWCP/HbgGgY5e04
RAtfCXJY7IlRMMpI4HZLZH1m/2ORK3TCqejNr9+ly8uxv+cqshEh5tVPfwqrEouE
++CvFDd1gKLZ5/tMo5VTD6T6aX4Rn0vgaWz3Z2xCIARZIOfH29iCjcUZShv4X0BN
VbSoMUDYKnitURmI9rJOsQUMTpPaUZslOQsWwdBGpWQpYBnX6TmvQYywNLOAOUcZ
Ystwr4BznyhQZKgRwaZ+a5rdp5a5SvLAN0jPdqGbC6av3/Iwt8P2HSKVaxC4Xzan
W47qYALtqlB8n15kJRExV6WToyFBiGL1jI6sYjQKoINNSO6pWtuZW7gTAJJejr8b
Ktgh/mrPUdh/gH5lIUH+CA1OL/hQHMEz7ZqxByTuNrmdc8Lx3+c+7rmlfv/xFU4r
BeaILBqfLo8us2DysMh594LoRrTruY4S+vuKZaUUgeqz58j/0bu7cV3lynXlvj+T
0Hhdz6Jj1yngw9z7maugj+rOHlprGXJwBty3VEvAlaUiNuZnZssJdX+Tyug0QetG
hFjm+A8tpnXR4dOOW5WEP+UtczXvjxk=
-----END CERTIFICATE-----`)

var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEArELt505QN90Hxj24ztPiHQ7PPdnG9wZbW2uKIDKj3JHmoyxK
HI/yZHyfnokIwbbXA+kiAbUebB+2D4C305QT+E+tHjTnsUpzP1bdcr3VuclzhjlV
UWEyvpgXsLl4Ig6WDOIQtHK9nOz6gIBzdG9twxJTSUcACMUv85zmn/qxjHYlF3sY
ABO1lIIQr3MRDVpBYooy0sJHHaSjKBP48NlRil6ePiTFlL79DxE22Bsj0HabythP
DFdIas8eNo5ONlM2EGdQpwfxf3jpqv0rNtFiKdrGbEi4X26rWiJ7wZwnj8eEz7tj
AMY+m800aUg3zzYfNlElTn0aDEg6tU6nlu5vRwIDAQABAoIBACS02lb4lBVjTv3K
NzAzbDI+7qBCYKhQvXTclIFJ6SreGCRbEqvFbKRG/ghdMPV+TZDyw6FTg9kMZNIm
3oUCP8Mgz0Xphhl8QNSVYPjLRNii+a/3VZvSt2pvpFSvIM85BnZWUbLx5D+lK7fo
JzH/cIcpx3+M1pAH3LDvlSEv3VeNl6Y+89F/UeZJLwCG3KEirypnUUYF/uC6rYQI
pYRvt46Bgdb6l+SWkTpSLDl9bwK4LL33HESx9MUyTBV+e6btRRqfdT6RE2gyVwxg
uq3gt3Y0XMQZa5YxOHG5KKzsPW8CbqcI+SBlsyQF4tPwaeVMppx2YL2L3SRiPnZT
9bW8b9kCgYEA4Gj2VzqSDOl0loNcNZSKgt/CKTIt/7PZOdUdxs9vKSdWVzLnqyK6
z8C8fiQxV5Fh+NK4xA2nOCICuLmxPRHKq4XLT1QDX8FxCUlh0Z7LtzKHPLvEPRqu
aWREVDl+5MxhgETEslU+8spIO4L5z4tLU4xo7jKp3OgybVKf5xhsNdMCgYEAxIKx
9RmN15FG55k4LmKyqjKp6A5TQj+x9nbCnnycHIEA1B1S8ELb8U8TP+LBspRWVBFU
8D5ZXX540Vq4RdtRkHbpOFY4UdoVKZDjUY2xuLKo2nfZ7juWuZyG8adfAm7t4Qar
98DCdeT3/LKNSZoVb7XqGYXhYM9CDrVnt2z2dD0CgYEAtXGIhBzSS+hSoQPTAWt5
1rmOpnpxIMdMwuriqYW87jxlHhoFoKRzAVlnzmH7Fz9wRJw0Uihr5QHyy2MwwBzr
jmWebiSSmdCxUX3ovnEza4tKNzvmPjWdgY9Vg/f89oed6fUwSLSOMgaGAsAytbF9
lS75BGcoWnnPk/7zVQm1LIsCgYEAmRYz6nw42tmLQjtD4Cb1hs+nO2eFhxO14QpN
vUfYGgCJk7UweomrbEas+VT+js8unZlO8UWxOrufBYFGEu2zkfaA42mPwHxDhjkg
TdUzwW41StSZixUS65A8NB+uTWf7mxUmfQDGvS9d3Zd/p/oIfxlZwP5iQJfVnz3F
CckyCgUCgYEAyrYQmz6Cfd5sQJmQ8aPO65h60ZfbM836So2CPP4JmD5QnWezMbtt
efLi/RF/9+KDOg9mGVi/8hrt4H5+wWMACIHQOM8selYRwHI+oBxY2Jn4pD4BhHGo
UHgtQ+vMxbP5Jm4jU9vB48JC7QkmV7PpUSb8fnd+nNca8mGIvMuzX1I=
-----END RSA PRIVATE KEY-----`)
