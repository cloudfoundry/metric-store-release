package testing

//go:generate $GOPATH/scripts/generate-certs
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
MIIDtTCCAZ0CFE14m61re9QIQ2+DtWzIU0624/NsMA0GCSqGSIb3DQEBCwUAMBox
GDAWBgNVBAMTD21ldHJpYy1zdG9yZS1jYTAeFw0xOTA4MjExODA5MzlaFw0yOTA4
MjAxODA5MzlaMBQxEjAQBgNVBAMMCWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEB
BQADggEPADCCAQoCggEBAM7DLiVYYs43XVaCBc0qCmPP2jubGfEXWUh6xAomjIdW
l6d0znVbdyGULnmjDn5TjINoPCBRZdUnmRLuOurpc+ww7XljVYwWAwtRrsv/jrUY
kte2wDM3b0JQwN29cGOiS3NFgwoH9a6+ID6vl2hvfFANpUtMEm0BjShiEbhRcsi9
nzgWicUk2I9icjdFZSYc3sBeSkgn3p16EV9hXna0GZBUq5VfLnqkMXZAPfRyZcBU
jm3qDeWgeXmc4nK+V72MNcGF4ldil5/ek2S1uCBG2O+y0HrBjxZZx9tNHII+cz4t
h8RfCVQ3fb6KRANyt4URWpM2LrDtLS2vCx6yalX+7+0CAwEAATANBgkqhkiG9w0B
AQsFAAOCAgEAsWVF+icPuYlitkAuHCOALszTT/PQbwAqe4m1Bk/KSoWn3rTMy5Jd
Xg0jaCWOOoppqtGG2khJIZt36tjFZdV1QO3Q2pK/c0SZMKf2hIXGpPt0Rvg16RgX
kWtZQRrnVf6SpXy0ZRsXTp+7TPw7edPwWmC4jVGEez1q3hSJkklCQiyYj3R+pkiV
xoKoEDFyd8aGjgwTbbRFx0vz10qSQcIqFRA2NWawE1bEJSB1cShfOgceGLixs1PY
9l7cRYxlUi80iQ2IMm4BcT6av0eURZdblvNnbusuPcKb0X1959LAZsWwgXEQZn+b
CqgUK/JfCAgg3+okyOvGS9aFGcBYEygzF2IDUrHIM8AD1vzVDRROuphNY5DpNZvW
TrpNBtuCFnLZGvIY9A9A2g7bcGx3Tdo08SAT5cHp7WD1MfZqnqTG0bq0TZZrg6r0
gQRRX0d2dFqpTgBwUZEcPSInB0JqVvp1ikqC7Os1I3owZ5lMufcCNXvzn5R6Toeo
GTXG8TOdE7w1IysJ0zLse9RD07NyZZvfnDrEigasAcCvkUilzgDFVY+15ZanEzrK
8r8fhGAiS3TXNgxWaR667AKgAYp9djoZ0LOgZmUUSPpi197P+9sQBTk2pJJ5KUre
N4HzpplZarn/magUytVhNoMD/cdZDxBJW42BZZwCpP1aEdmnispXUew=
-----END CERTIFICATE-----`)

var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAzsMuJVhizjddVoIFzSoKY8/aO5sZ8RdZSHrECiaMh1aXp3TO
dVt3IZQueaMOflOMg2g8IFFl1SeZEu466ulz7DDteWNVjBYDC1Guy/+OtRiS17bA
MzdvQlDA3b1wY6JLc0WDCgf1rr4gPq+XaG98UA2lS0wSbQGNKGIRuFFyyL2fOBaJ
xSTYj2JyN0VlJhzewF5KSCfenXoRX2FedrQZkFSrlV8ueqQxdkA99HJlwFSObeoN
5aB5eZzicr5XvYw1wYXiV2KXn96TZLW4IEbY77LQesGPFlnH200cgj5zPi2HxF8J
VDd9vopEA3K3hRFakzYusO0tLa8LHrJqVf7v7QIDAQABAoIBAFXmyVlCq2o5nlG+
m2JtwPtO88An5FNB/Bocxy3gbiocU82CvfQMGCafRd/LWs4pMAu4VqKmrsQsO3Ce
AWRvsXXDriXsmzIkQweE3DZs6oFawEdW6etdcKAApOB7QCJk3yv5CUQ1omEDJKpm
kWUWTHOF99KcvFsFdfv9IpeNXz7+qecq843k7F2zWUblm1YxC2cmCYmCFNJv0fBW
8D6nZgTzJmv/nTCwfzg6Ykxai4DiLjFFaBUx/4GG43QuNCAlExmmHmJEojVjWHXj
UeI4NVK9yBzrdvUXj642Wp/71iHQv+L8Itc2j/4TeUCXTU5uLHmKs3tzpSvAd6Gh
v/tr0AECgYEA53QpP1NAerntAkoe3GLETkJUdIE9DOL3y47r6g1kKHF9vMvBY8fg
VH0llObLpOTdk1ba+Pvh6Bn4hMP/DOPaaC2n5UPOpEI6PIeTpYKfi1Zk6HPmY0Dk
X55Mn4voJmWe9VLtm6oJTxRYs7ah1Q5AJ0PtFipiCdYovzky3ZQBgu0CgYEA5LCp
1Vy2zTR/abzL88xeTbK207Z8arbkdu3l3E7qITfgMLkWULtJTGZXX8Q7YgOwmwGD
Wp1N/98TAB8uO+91X59wZcZDsKEsA0lVmSn8+q6snWyNlhwZ3HgQw8G8zPZHLOcx
0MTU3rQW8Xq7BC66FOMhtdIunZy/BAHXicaUgQECgYAb1Ts0k0VYvM0EjndBl1r1
8kIHtJbr2stjni8+eRfHSUaOko4R+rI+VsJTMqHglWkT08kHUfrrl1vsU0lzel8E
UiEzj8DkvdYU+1TE/X1EG0KNNYrJ+r67xOR/9yoWm/fOlodeRcdSzCaSje7OGSWb
0y5KkRQzDJ7fx/gW7zpzTQKBgQCmm9Z/bTZ4teCFplhoW+HwdV0hTPfDv08fHh6y
rIOCg/S/SnjphCjYkk7hpFMnC00lAKsz3xquaVSsaAsE+2XlroDyhMlX63PnSQwl
tCNsdsmnPyi/zeVBa++6znDAWkRsgFsYn+39+fIlJ6cMWwaSpQ8wKdpwVXwMbVMc
OyKCAQKBgAZjXoTUgxWIBxTEg7DrrOJOcTwzkikXfb9heww5Scy8mX1dDuH3ngMX
CWBRqIXYowQ0TvQiCUXG42p1NEkLnvH44efadvUOFvNpbIcm6fUB/DEa2/LvDLQB
xVbWi1Lx4B2qx5eOfG55ep47fZR7+Hy7+Zb0E9RzSbQTLbccgppi
-----END RSA PRIVATE KEY-----`)
