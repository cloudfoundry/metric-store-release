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
	log.Printf("MutualTLSClientConfig: %v", tlsConfig)
	if err != nil {
		panic("could not create MutualTLSClientConfig")
	}

	return tlsConfig
}

var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIFGDCCAwCgAwIBAgIJANCFTxfmfZSSMA0GCSqGSIb3DQEBCwUAMBoxGDAWBgNV
BAMTD21ldHJpYy1zdG9yZS1jYTAeFw0yNTA1MDgwNzA5MzNaFw0zNTA1MDgwNzA5
MzNaMIGFMQswCQYDVQQGEwJVUzELMAkGA1UECAwCQ0ExCzAJBgNVBAcMAlBBMREw
DwYDVQQKDAhCcm9hZGNvbTEOMAwGA1UECwwFVGFuenUxDTALBgNVBAMMBHRlc3Qx
KjAoBgkqhkiG9w0BCQEWG3NyaW5pdmFzLnN1bmthQGJyb2FkY29tLmNvbTCCASIw
DQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAK3LwliEtuSrVBHX86cH2trLC8G+
aHBsH3DRVQyVUBdSjjHoQDDMWLb5aON8WhTPd8IdMe3jewYMYc4Cpw3jSLSrLdA4
GO9j2qq07jqPHp44KOM6tJYl7sGiquwGSjWbSZKtsZQWwNDbMTzPqf65y5VJnYkw
65PpAmI0LOdEaSOUSr5C5aMsubnvfyJphmlTPzcTY6IVwBLdB5mJ7OTkfBeU6YR0
apMUeETpa3yhZAX+NbQzvmZTVFD3r2bl6AG1Kpn0WYQqhmRmdV86sUtvI+i8UZvg
Oce4MhKuoS10xB/UAW0+G4uLbappraxNhbo0cEPP5gILmaKes+k4r6D+yBcCAwEA
AaOB9DCB8TAJBgNVHRMEAjAAMBEGCWCGSAGG+EIBAQQEAwIGQDAzBglghkgBhvhC
AQ0EJhYkT3BlblNTTCBHZW5lcmF0ZWQgU2VydmVyIENlcnRpZmljYXRlMB0GA1Ud
DgQWBBTJ1roqtfQQj9Zz6tp5IqMTXXCyGTBCBgNVHSMEOzA5gBQgg2eV6YsKEFpn
m6uqPotP0Z7bs6EepBwwGjEYMBYGA1UEAxMPbWV0cmljLXN0b3JlLWNhggEBMA4G
A1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAUBgNVHREEDTALggls
b2NhbGhvc3QwDQYJKoZIhvcNAQELBQADggIBAOPLMpNzmxs1fy44ODrVyjSru6ho
gpkmJL7g2bvajBw1WyHbouP6MEdiny4gbhJv9TeqTS55g1lTbOABc2QA2YlBKIy9
rrnuMUhEpLCKYFSHN0q3FLdmTWRTGoORYjyl7PchK3HyQ5RTAsges2PW6ZxyEXOd
WrjSD/6ixCjv5/LvyJ1FMKkl1N6Nft8dTnHDzuxYS85RYkjikOEa0C7hnQHrEpaW
Iz02pBemAAtuoQnKX55Tj2AfHVfr0NSCZLzpQ1UCIVVQBIUiE69ZZep9uYO4AFn0
yaV6b8nXS32O5QpNHOVCPIot0QW7pEeSVUKU4aI9CebZRq2VLwpY8ya3j4NicLX/
pZIsryCiML8s2kIxsuOFqKzoI3k2biaIInjnFeiW6LgGn30hwK22yTopqRL3hRk0
APhc/iA+R51gFZPN0+UuRpbT0RfsACQNpK5EjeLdX2RHjjBlHgZDfzzZKXkwjPnj
h+RJwBzDkb24NNRQa/6b5rMF0UK13MW5m6quOxMaTWgy5XDp9Cq1GvM8JfblcQLK
sP1Hxb9kqEvoYyQzpd/2a5F9Bm6Ag7fFYnciljm7Wm9hEwxj16gGDW09Z7lpijS2
VKx7m2nxli6A+JlirGXCs9X+Qe/hYHhMV0PilBeCLcj2wjFgdp+y4gYHegrHgSHh
F4NPeKI7FP99ajsj
-----END CERTIFICATE-----`)

var localhostKey = []byte(`-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCty8JYhLbkq1QR
1/OnB9raywvBvmhwbB9w0VUMlVAXUo4x6EAwzFi2+WjjfFoUz3fCHTHt43sGDGHO
AqcN40i0qy3QOBjvY9qqtO46jx6eOCjjOrSWJe7BoqrsBko1m0mSrbGUFsDQ2zE8
z6n+ucuVSZ2JMOuT6QJiNCznRGkjlEq+QuWjLLm5738iaYZpUz83E2OiFcAS3QeZ
iezk5HwXlOmEdGqTFHhE6Wt8oWQF/jW0M75mU1RQ969m5egBtSqZ9FmEKoZkZnVf
OrFLbyPovFGb4DnHuDISrqEtdMQf1AFtPhuLi22qaa2sTYW6NHBDz+YCC5minrPp
OK+g/sgXAgMBAAECggEACk7xP0bZgkUXPqey1mlrQo1gkr4a4iNhPTo6JsqEbmqB
iW95JZ8GDad/E5JaO+AoyxhOnQmTg0K6WmAvgG3GvJ1nbj4/KS1tchduBrmOdPWa
/aW+jr+/omr/lr6yXO04NyPOSjZY4XDe25IfRyVfo4JmS0jd8Dv8RDVMCu7OPanp
Ab/FuSPo7elnQvZJ20IPHt5/WpxEWz/biViQbgWLWBgICAlV+KaumElAktsyV7bt
LNjq9md4fznZEwo9T3+Mzzrn6GAa4l8Hww/xBw0JAJ7kWQYNjWSG5cY0+uIv8yLA
6Xlf5/QRv9jijPd5i5H/uTM5uHy/fUsyO2O19wlwYQKBgQDdJevtTWEFLndyT8OB
9G3IuZb9Nf/CpbX+01KJ1Ra86gjO0wdZV19LK/70vDXE9QOjPuLQZPt3KMnTfZlF
hJjivpneP4cL004+4mWKN2BuLY9n4RqVmMQ3779hRn+bJwTAGiRTdK0nrxSOcCzg
3qkw3keWQOVG07ZH24DEgUN+SwKBgQDJL3GccZySwja/dMQMDAEtTR6Gl8mO7G1Y
JSZApU0aiFAxPPQFdXYAwE77/JpGT1Pr1AM7E5H+9i8ajIMkMSI2wvqLfW7Xf9h7
DVECyqLA2BtbiUzSRGrRJ0mVE20zym0ZNzzHhxcqzSoT8YyW9XFR8SqMoBx5B2u/
BXoU4X8N5QKBgHaoTjp5dkEteXGgUqp72BwHWHhsbNqnx0r/YB4Mc7LRcABpQlwx
gTP4W0g9ZCxVuqnwqApg5Hw/KmuLzJ18U/v1gOG6/F7f9e/P0eOjat4zG+sE4Rq7
aS0KOombJgS9ntLkM/GDfRT53/G9RpcxYV6TJZ39HAgwuHE92Y2WPfyZAoGAHK8n
A6cvK72FEMcVLKKJiGv2bjo2Aqqy7F5fldf7pkzJIjwOjriwmmrQ2Byr4lptHLKd
w06HAlMXZDGkgQSAXE5wanL32sHfm6vKYRuDGPu26tYondIjaK6xTw/2Aexaob2+
bLRWGUQnO7C02tEj1wsLhgFODfOA6TterJt6AgECgYAOfikOyVcRObpixwrVpMOt
BlYZR1IBVtYJatj2CodovC7UAm/hvljzjhN86yY5yD6sbnh6UXOhRwf0P3APBIzs
wiwKncUF9ZcSSqdxUrIkU8mH4MAu0smRrNgpTSbF39rMjoIMr4vYV5Ix0f6bESu+
wEItee5AUdWC/lGzGCXMpg==
-----END PRIVATE KEY-----`)
