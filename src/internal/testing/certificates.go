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
// > openssl x509 -req -in localhost.csr -CA ms_ca.crt -CAkey ms_ca.key -CAcreateserial -out localhost.crt -days 3652 -sha256 -extfile ../ssl.cnf
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
MIIE9jCCAt6gAwIBAgIUMcPnQpCYgH98iK0DgCpA7sFA11UwDQYJKoZIhvcNAQEL
BQAwGjEYMBYGA1UEAxMPbWV0cmljLXN0b3JlLWNhMB4XDTIwMDkwODIwMjgwNVoX
DTMwMDkwODIwMjgwNVowWTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3Rh
dGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDESMBAGA1UEAwwJ
bG9jYWxob3N0MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAku0czmve
ELviE8MGUrDkJob9JFX6SlNdMPfKSAjQSZ7+Y8K+vtjXR2co9ysTJ7B35ps7Q1L1
0Z+JP38gukdb/1H3IJgBGIfmIj85yDFjDF1uSFKrNfLxGAL2Dx2aVvO7l7yNI0IH
ee0dpNZo541uCl40CEbrWIDm9bGL7t2yVK1x7qND4e9XCG34p8OYRhqTe5gXblEN
ysApo0Gxem/kl+uMPXmjSZm5zRyMClNqN8yUBLbULcPiAcsYXczhJmWHwElUlpx9
K52+1rfyu/rG3hKKh+XySp3Ykn1EDxIHJ+pgkkoLAwck8iYrnUImdHispSjFaWZ5
RcIQPzQdmKBTHwIDAQABo4H0MIHxMAkGA1UdEwQCMAAwEQYJYIZIAYb4QgEBBAQD
AgZAMDMGCWCGSAGG+EIBDQQmFiRPcGVuU1NMIEdlbmVyYXRlZCBTZXJ2ZXIgQ2Vy
dGlmaWNhdGUwHQYDVR0OBBYEFIcNKk6DQr6wpAR/JXL5rgZVtiYIMEIGA1UdIwQ7
MDmAFPXbp11LBYeb4eF6lfEa2KjNJp5boR6kHDAaMRgwFgYDVQQDEw9tZXRyaWMt
c3RvcmUtY2GCAQEwDgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQMMAoGCCsGAQUFBwMB
MBQGA1UdEQQNMAuCCWxvY2FsaG9zdDANBgkqhkiG9w0BAQsFAAOCAgEAl4a45L4V
0UK1v2SCCUlGap24opZiH8moxr6+unR31SCGKVu76WJEWjip/l8wP8tujt9me2Xo
8R/Sr8mBh75YWIzXLGfl2ALzLDyD43sM7odax+OuF+XQ170/YnELjwLNHrVE+2fC
emMEysDVJn1+27+em3S1J1gLsE7+pEKwnl1DmW6SZeNpWL5+A3vzGQxs8KspLsbB
V86FABDEIvvgTZKB2Qc/vA11RMZsWHYUwCs4VNUtWj3f4X/1lRQrUhsZaMY5OFyJ
p+XLDgFKu7N3eC+2bbgo5SjC5/JraR0QaaUbrIEvSi5iXmn7AQNImoM0+0sJIPRt
XN/7XOCPi48dxHlczDDNgHcVFRad1mw21PaVv4n6eDhnJvf/lUHZ1oG8t1z651dx
bx4wLcwcBzgLdOUSwgKzpuTi+0bamClDdMsttIr5qHCHGscgp5LtMxpNrp6nj+zQ
2hw9LcV0+asHkv0XNJoLhdOLCHKZkDPURncW6Os3I9PoOmIfNSu+KXA2EfY8uRwq
AqVzFPLsOJs8Po9Ed62TRZS3fTGgqgOOWAjZzyGC3kxMB+767zz1H7hOaSSzhQ7n
iZxSeI4nyhNM7bm72OFTjeTtLgikXl27Hoo40DZ9RlA78avL6GPnfRMJq1MUB6TG
qNgHe4RrtQc/my9b4pRbnlFMzt+fvuLc7pg=
-----END CERTIFICATE-----`)

var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAku0czmveELviE8MGUrDkJob9JFX6SlNdMPfKSAjQSZ7+Y8K+
vtjXR2co9ysTJ7B35ps7Q1L10Z+JP38gukdb/1H3IJgBGIfmIj85yDFjDF1uSFKr
NfLxGAL2Dx2aVvO7l7yNI0IHee0dpNZo541uCl40CEbrWIDm9bGL7t2yVK1x7qND
4e9XCG34p8OYRhqTe5gXblENysApo0Gxem/kl+uMPXmjSZm5zRyMClNqN8yUBLbU
LcPiAcsYXczhJmWHwElUlpx9K52+1rfyu/rG3hKKh+XySp3Ykn1EDxIHJ+pgkkoL
Awck8iYrnUImdHispSjFaWZ5RcIQPzQdmKBTHwIDAQABAoIBAGx26QYmMYiO+yX1
mmxfM/6RNr2lTyGhizGEK+ujvggrfMcu1FvVfo+yw1Y8kWaCavFt9YEM9HXs3Yhn
lESQO4UwAE0qidyPLsBnhoOYmfNd4fU4OjaYg41jWjzscKzyP7GTu2mk7BoBhxnS
Qx11lh/HTYgyurjaaCZHDPOo7GZ/ijr3fq2ntpcjE4sJ+oww+f3e/KyOs2T2xZID
nucDYkCz0H6t+WkIB1NZ06JMtrVkx8EDxa4lntAQlfTNDBt4tWFsmsBqDcFDIbyV
UlSb8NAAnUNOKhZLaZFbIzdSgPnFK3RzffUxDc+Cb2tyineTJG4dPGYVtdQ+fZ9V
9Nr1j0kCgYEAwwwrJROeU2WpsbkVci1Rjy9W3mIY7aLvwvjZnSqqIdsETh32wyqj
VgJnzVLYFpSLWWQy5vpogPx7ekfW7aaG/0matpqoaioC6B9P4lnnsptbGZ/HVrxf
0oPOI6bZvGHROP9oPgMEO6xOKmBndvDFUgU3cnS1KMXPK4Ffo3EwvPUCgYEAwNc7
XKEKr5AGzSmLKM90hL2RnHa17AmkWmghsG3WGq3tr7xppAAznStFpfKD63d3Z/xD
p4OhU0k+jC2ImzNEgifXYBYJEcF+4nb2ifUHf92PVNIcUzMTQ983k5NqHnZ2giXD
sjuc5kvOSPe3x3UGRkMqjA7maHxT7TwrCCMIA0MCgYEAuVMQk5QGdXs9N46CoBAL
VJ9ITNisku6kHX9RVZI6QRzXBFpYIqyasZhrhJ3Yj+voU7Fu5CS9swEzWcZ0oy39
Swdmp2Zf6y+rv14xCwMdsYkHkFtOjk1CRsXR0fIxJFtU7nFPXMyE8yYOxxj+dfbj
pcErI+dcUjdRVyb/9k4QXxkCgYEAvx0Pka9wXanJc852Qc3H1j3hb0Ue6nxry8Vj
jexswigZZY6+y2//twXJkrsUDmpphj619fA4E8iFGnuN5PIcNTmy82EjP9uaGmp3
zyVtd0Vm5VMOC06VCzDPbYwtIV+Z90Xh+mOmDX6XRC1sj1nlDAFIy2GA65A7xONa
akf9iu0CgYBKeLQLqAr4gjLbHdMWJp7noEkX3iYNJ5BmZga6XbVeK4IiRMrBjpmB
AJ1N5PoKlmtDlAMlqW84eLCs292FhLXZJFV2ZvhE5EQ2HRO9JCUhwdqVSi1zaKr/
qwAoa7WdESITnB6GXY62ubae3OKBxlRABbEliO/YQsE/l2E+p2UFfQ==
-----END RSA PRIVATE KEY-----`)
