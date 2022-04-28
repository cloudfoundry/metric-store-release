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
MIIE4jCCAsqgAwIBAgIUI8iSzXNezM6iOg5vtRBhZn3mzUkwDQYJKoZIhvcNAQEL
BQAwGjEYMBYGA1UEAxMPbWV0cmljLXN0b3JlLWNhMB4XDTIyMDMwOTE1MDcwNVoX
DTMyMDMwODE1MDcwNVowRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3Rh
dGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDCCASIwDQYJKoZI
hvcNAQEBBQADggEPADCCAQoCggEBAOqksMLdWozNxrD1QoESMcsq/AgGp628mr6C
zc6D8xmxOE/fdeege5JBCSxuDvz8DxJI/UaDrbDUzH72X8bcsVRGf+hYt9dpnZWb
UCrumJsjLQ2vX3TuypEx7mTdx4GzYaLgfMDf2fi3m3ny8iggwt529+45GRgQAxG5
rVqvl5VYMLWTnMXUGJEGBGSeBOii6lbzJ28alaAh/gd7gDiTI1JLpR7RLm+YoLBx
VVC66yNlQ4r8mov0xx3b4kwawgJK35UiH80APwzzIttFCfgnib/BIhZQPCHsNAZz
g86CmPhnnhyw67wMVO1+U4rTv39LauRb2qG+M1sHyRakwWo87V8CAwEAAaOB9DCB
8TAJBgNVHRMEAjAAMBEGCWCGSAGG+EIBAQQEAwIGQDAzBglghkgBhvhCAQ0EJhYk
T3BlblNTTCBHZW5lcmF0ZWQgU2VydmVyIENlcnRpZmljYXRlMB0GA1UdDgQWBBRt
d279oTFLecXqo1YRvzMt4SxQnjBCBgNVHSMEOzA5gBRQx2ANY3ddbypAOvQQVwOm
ZVmqDKEepBwwGjEYMBYGA1UEAxMPbWV0cmljLXN0b3JlLWNhggEBMA4GA1UdDwEB
/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAUBgNVHREEDTALgglsb2NhbGhv
c3QwDQYJKoZIhvcNAQELBQADggIBAD0qSeQjbYdQgootqizqYcvpgkWX0CKg9raK
JwRiYNxhbuqtZ6huYeyxxou4WuebK/mO2Suvy6MSBydb7g9r7rTwM681rWITrobx
udzEpW6BScs4vEIPXgYURsp+eOEyMKL5s6FRY3C6yD0tYQnvjzFx4pOB8eyCVHwD
9ZCm3OZDT+acTWc4i0nz9WIQ1nOTwNeF/VJtQE9FH5yIJ1XESaHLlcy+8H4nIeec
w8NBiVlO4oLcK3HWaYc+g9mujmoB6o9kLY0FO44u/Rt+X/Gcyv6teJNc0Eu7fhgv
3ABuiZ0QkoD9DqBPIRNZf7ruafxS0pP7j93DxoNMYjaYjlXZdN0jUsLWJeW0CYzo
VJ5f4b9X9DPvB3Migmcwrb2d9SPfnTmp34CdVqHRDnRIol99VXT5ilR0JERmY8Bq
aFKhWHPzNEcRzii0LTcZknlcz5aQbPyQlXTG3kPHG3mAV2kSQJIa/FbgPtAjVOwe
ZPBCSanpVaBP5ZvKK6Q6P83ycovlzEdzrbAApoAy7TA9jj8wGlm1Y+qtr+EnV/6k
pWNFvRTImjB1drpKXpHNZg+F8NBC3SRZicdf7C1WOnXS18i6LT9P+s2SBwIqBP04
ndr92NJF3OgurD6NCG7eIf4hSOxVvNbEtnfsJRYI9UKMFRMZ5O6uZQP517+0uwGC
37se3AY6
-----END CERTIFICATE-----`)

var localhostKey = []byte(`-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDqpLDC3VqMzcaw
9UKBEjHLKvwIBqetvJq+gs3Og/MZsThP33XnoHuSQQksbg78/A8SSP1Gg62w1Mx+
9l/G3LFURn/oWLfXaZ2Vm1Aq7pibIy0Nr1907sqRMe5k3ceBs2Gi4HzA39n4t5t5
8vIoIMLedvfuORkYEAMRua1ar5eVWDC1k5zF1BiRBgRkngTooupW8ydvGpWgIf4H
e4A4kyNSS6Ue0S5vmKCwcVVQuusjZUOK/JqL9Mcd2+JMGsICSt+VIh/NAD8M8yLb
RQn4J4m/wSIWUDwh7DQGc4POgpj4Z54csOu8DFTtflOK079/S2rkW9qhvjNbB8kW
pMFqPO1fAgMBAAECggEABhQEEYLYZfitTUeH1H4bKhnP84lF7gXftaWqcnoP4bop
15qAnGJs27SxlHlkDAD03FK7CjOTsUFMltFdA8gTE+LLoheCPk3vFhay30R/ptfP
bUuosQe/2ZOt4Duv1QKc5IePfmjjZZekAGcJsikUbdzZE/kIrGCQeR9n3THalF9q
wURS4pVZXGtO3CAbPeMxDL28N4o6EoYciVPJiYMvVV2Q4XPLZkkYRorPf4MJpC2p
Bhj6g1UY6hvBmXkJuwRkJFPM0ed7HQ1djfJzAY8xJLI/uGYeQEHkuMvte5BjR6vS
7Gt5o/17A8geGaoHZLgq8YrwKrtw6cuyhhi4db96WQKBgQDr5Rtm7zzWc4S+NX6c
tEZpJPew8HzJ1H999hXBxpZXHfv+wd7gniAFzoGPwQSDOPR6SAgovJlfB6qADpOb
M0S3vvNRx6+kICVPQlq9Z0ac9W7jcM3e6yX3kZLH0jsJhRI9T5y8sx8xapLjHM6z
HJG/pWxvL34M6FTKbhnHqimitQKBgQD+pEZTDmZotBlIrqZ21STrxG/VM+flHcpZ
8329pWZ6Y+1lEOTn0X9BhAdKKYsaxs6ze6dL0B6ZUMBH96n0Ym7iT+7/BgYCVySX
1dX/wnXa3M9Xh9OyDhaQGBdUkPuvAhIVhHx5gf8iA6Dp9A6j7Cp23O+qIFPOmWlo
pfFIF9j4QwKBgQC5qy+BOmY5KN1fVP2d26rb0VP/eZnOxim++/Ut+t+UHC6e2vtQ
8kSkLWD+w96IZkjPAmkhnyhcis0hU9fMPXMl6O7c/H37ga28D68aCvKiUe+ApuXz
QkQ08uiDzK3ZFVtA1Ku6PoYbwBVzGtZ6Vc9F9688aDYfdLJgTn6OEBoLTQKBgGci
whkBzBi8WUFG+8VFrx6PAXyo+VOjLUIhjNwzEb6gmpZEsXHzOEeg4hL3oI/H1hB8
FNZwBPSz8C018nA5LhAbsAE6v1RTV07oHTTp3jI3HQOmz+deLWVPXKOz1Tyc6hYt
Av1z2ZI9Rf98CjH6hXh/I9MUJN2Y5UitbXx1rDthAoGAPS0P6EfrSVQzkpnaCD6B
HGUi1bDH9rqAO62ztNLZ3pQ1c2ViEnYcwG0zJj3bFSmufUlerMQLjsulBOxefJJQ
2svGsse8jo5Ke6m6zdZM5bJVoeKLlhPTdIqw2HaEa159j8vsDA4yKVIm5G7o33iH
sCllQE0CjkIFb/zbp0B8v68=
-----END PRIVATE KEY-----`)
