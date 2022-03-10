// Code generated for package testing by go-bindata DO NOT EDIT. (@generated)
// sources:
// certs/metric-store-ca.crl (934B)
// certs/metric-store-ca.crt (1.777kB)
// certs/metric-store-ca.key (3.243kB)
// certs/metric-store.crt (1.545kB)
// certs/metric-store.csr (952B)
// certs/metric-store.key (1.679kB)

package testing

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type asset struct {
	bytes  []byte
	info   os.FileInfo
	digest [sha256.Size]byte
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _metricStoreCaCrl = []byte(`-----BEGIN X509 CRL-----
MIICiTBzAgEBMA0GCSqGSIb3DQEBCwUAMBoxGDAWBgNVBAMTD21ldHJpYy1zdG9y
ZS1jYRcNMjIwMzA5MTUwMzQ5WhcNMjMwOTA5MTUwMzQ4WjAAoCMwITAfBgNVHSME
GDAWgBRQx2ANY3ddbypAOvQQVwOmZVmqDDANBgkqhkiG9w0BAQsFAAOCAgEAGjh8
u3eRPs1wOwYb/mk7CX4bARvK1wtrMKmZ40FGYgFIjZNi/+pO1JwhQxmrWnirjcdO
960Om5Ol8+iABWuEVe41RdbEERTsfg72m0MZwSVTkYhtnnoMU3qYr0/J4aopvD4u
IRMtE0TSrz037O3AJUePqrcqSTZEQVgPSco81CYLakVLUrffPhL7ejndEbwcKMvT
xh1eFol0NvApsdThf3E5UdcEENBqVTRTj0bBWxK+qJdDT86zTJGE6hIGsi2/WVqc
9nuo59hUrHsGoQTHDD8Qk+Q9VuqflKNqkHjpfTSKzZCHuVW5/VoVXt9g+G2PCK08
IiG4X/b/nlPKEz+nT6dYJO/VLWIrIPEClz2LWqsLUEsjBSmK/r3zq9sYOUayBAn2
OaCnYir8+9YZYhyUhYbJ3XYuXMolVof55reR3gXN2/27wVvf84pdhUGjc3l7PCaX
TNcS96hLdc7VeQXjkrRUaPsk5WWcp6Xgk6NS/gcnnMP7XciNBbyBqHtbg4mCAMaE
vcWKiwBM5g6ZsDjjDQthGAW1bLKCRn8tIeqKApv/e+sVaG6Axfvut8Qsrj92vUfA
iioWmuA/rEARPHBTOKIuog9x6yjxEC7maNVMQApnXYo/nQjj/n0Q0cUTZF6rLCz3
7VpMtEfi49jhKk5pzfPBLHwRLtZr/yeR3la/DYs=
-----END X509 CRL-----
`)

func metricStoreCaCrlBytes() ([]byte, error) {
	return _metricStoreCaCrl, nil
}

func metricStoreCaCrl() (*asset, error) {
	bytes, err := metricStoreCaCrlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metric-store-ca.crl", size: 934, mode: os.FileMode(0444), modTime: time.Unix(1646838229, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0xc8, 0xc9, 0x9a, 0x7, 0xeb, 0x65, 0xdc, 0x2a, 0xf1, 0x80, 0x58, 0xf, 0xf6, 0x4c, 0xa6, 0x7f, 0x9a, 0xbb, 0x9f, 0x23, 0x65, 0x1b, 0x10, 0x74, 0xff, 0xcc, 0xbe, 0x22, 0xd1, 0xfe, 0x68, 0x75}}
	return a, nil
}

var _metricStoreCaCrt = []byte(`-----BEGIN CERTIFICATE-----
MIIE9DCCAtygAwIBAgIBATANBgkqhkiG9w0BAQsFADAaMRgwFgYDVQQDEw9tZXRy
aWMtc3RvcmUtY2EwHhcNMjIwMzA5MTUwMzQ5WhcNMjMwOTA5MTUwMzQ4WjAaMRgw
FgYDVQQDEw9tZXRyaWMtc3RvcmUtY2EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAw
ggIKAoICAQCvtwbrXjFP7VSmTFOPUi4uL/jPgg4aQhHgBVCqGVpOgjIDSBkj1IzB
ePyZe3jcFmm4GN0l4Pnuiy1kqRgYYX1tiJ2B/tkkUthPGlenDXTnu3paW6+BFTKN
as7+4fk7tlfSH2Kef5BQqhbDyij5tDdpqOL/9XYHZ3OaYRhytG2WvnnXWm7jduZZ
ldSvwLF50HdKJ0RCC51ohJkyXK9MBpF6eQdAEV5DABmmpFSBaFEqXi3V+8d3D5vF
c6wGrm052DwoZMzDwQKZEtTZjlkqMiHfHN6YtC7zCdOkSuTrijmC2Y9VIJ3jJYYr
/MWpZTbVLgIr6Tmk377z+ayZcAqNY0z/0Ojl0cRzj6T+NHYHB7t1rZW2r+Ujusk+
PKqj2xao86SaO8Ftro+ggIyGqjHSMNiozrvh6jvmq1rFuUn1J5IoWV77zDpFSq61
HgQJU9HrWjZl7yrGMuQArYMRwMq2DkxFnnZA0APxob4ik8cfaUccIRfEmN/FQXRJ
oTOIk0hKyjni+H4CC/l6tQfwA9s45dfpnaumUValBwE1BYhO3VtmbQ/evNh//aFu
HUtgtyKGrS8VugfdhKQtIgW6wwbStkLFK8srPHdPtfMR+jhb7cR5tzjiPSi6/pId
N/Ln/5Is8KPAW8y5nEeODAZqP7EMEhNYn9gVwwG3aHTJK3fpyQaBtwIDAQABo0Uw
QzAOBgNVHQ8BAf8EBAMCAQYwEgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQU
UMdgDWN3XW8qQDr0EFcDpmVZqgwwDQYJKoZIhvcNAQELBQADggIBADRh6s4NV++b
AXwSADBn4bVBJJPkIjkgIpL5lvi8qvNeEvpKRCAnhPctIOk7OeuKbygXwWiz/4pv
Ym9rO7rV6bXAtWfGheMLVH2qG4xRyzMs8qVM6fF658iC11JEB43q+7D93IeS+bJO
6JqZpzNNsf0iFKvu3HQ3F4JrqJ7TLykrlT0yajMe8baf3dhh4wVHWEvWNapfgGqf
BSASRdnODr/i8lf4sLMMvr8ngJZjH2RcZ+QzjsDS2OxJBvgsam1zWHxHLIsz7DJf
TJ7RgXk5FKi6fqlI6yCe2duSoLOq5XD+JoCWj4eqiaDAL+zM/tVa450zp8uenDi4
HM3H/bjRXtrJOftWEoCNbojxL+W02kLy9caZDY4kJdDuv7SMFWJ6NvURhBYMxMH3
l3UU5e21B5hoIeMpwQLPK9kXRNIoVyaNgSsAxuIH+e3j3rDs23kPbzbT2ZyleXjL
b+hF27U1UgqcjRuygFQBqg1ktW6xgSJVtZ5nK1XfUV2yRltcEONCDBmAeWs0ngQK
fC+Yk05jLixEEXyctPompCZKhcJ8pILfdFdlboi4p+GkhHVZy8aprQPdDbB+QJZy
X3gU3VC7g9ukmfBIhEDOKEDXKLlVmb5WIsU6wYj1HuK3drAY7Z+RAWn0gNrNaGH+
Do1UYd4MMINEUDbfYyT7agie+jwOa9oY
-----END CERTIFICATE-----
`)

func metricStoreCaCrtBytes() ([]byte, error) {
	return _metricStoreCaCrt, nil
}

func metricStoreCaCrt() (*asset, error) {
	bytes, err := metricStoreCaCrtBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metric-store-ca.crt", size: 1777, mode: os.FileMode(0444), modTime: time.Unix(1646838229, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x4c, 0xe1, 0x35, 0xe4, 0x67, 0xbe, 0x4c, 0x1f, 0xf9, 0x95, 0x86, 0xea, 0xf8, 0xb4, 0xb3, 0x8b, 0xf8, 0xdf, 0xea, 0xf0, 0x31, 0xb3, 0x70, 0xc2, 0x20, 0x1a, 0xf, 0x7b, 0x29, 0xad, 0x20, 0xc0}}
	return a, nil
}

var _metricStoreCaKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----
`)

func metricStoreCaKeyBytes() ([]byte, error) {
	return _metricStoreCaKey, nil
}

func metricStoreCaKey() (*asset, error) {
	bytes, err := metricStoreCaKeyBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metric-store-ca.key", size: 3243, mode: os.FileMode(0440), modTime: time.Unix(1646838229, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x73, 0x89, 0x9a, 0xb5, 0xa7, 0x33, 0x1e, 0xab, 0x36, 0xe5, 0xc6, 0x5a, 0x50, 0xde, 0x17, 0xe5, 0xa6, 0xd7, 0x8d, 0x4c, 0x1, 0xca, 0x3f, 0x16, 0x14, 0x1c, 0xb7, 0xe2, 0x6, 0x59, 0x4f, 0xc5}}
	return a, nil
}

var _metricStoreCrt = []byte(`-----BEGIN CERTIFICATE-----
MIIESDCCAjCgAwIBAgIRAP0H8e3qwRaiPhUQfRTtTmUwDQYJKoZIhvcNAQELBQAw
GjEYMBYGA1UEAxMPbWV0cmljLXN0b3JlLWNhMB4XDTIyMDMwOTE1MDM0OVoXDTIz
MDkwOTE1MDM0OFowFzEVMBMGA1UEAxMMbWV0cmljLXN0b3JlMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtqB1jT7jtkJ9cUZk2RBeAi8itAO3A087Gwf0
3wMw4BTRC0c86yi0dDe12sO8Lbm08DmUI3fUStsMttym65vGyUkEcdnNhYTvUlMb
W3bkjyvs+YZ+RPiKoF/McgGZTBGxZ3i/+wt4HA6eIhzBjMFvB6cE/dwV308xVswN
HcRkQOBj6n98lJUImUAmCLJ1I1WCZ8Kyu0E+ROuzQenM3zKm7dI4f0wpezwcJ/U/
wOCPnEzKH/fYoFBpvcAibTh00IGctLWOKgkdH1kRklfnzzSMein3LyaKy85p86UL
5IMEb2Ic0kAMhvNL/XRb1jF4x4pfzNVTflPukOx0bJkCn0v5+wIDAQABo4GLMIGI
MA4GA1UdDwEB/wQEAwIDuDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIw
HQYDVR0OBBYEFJ3Vn5I7GFTn0IQujTdvxBvCl3EeMB8GA1UdIwQYMBaAFFDHYA1j
d11vKkA69BBXA6ZlWaoMMBcGA1UdEQQQMA6CDG1ldHJpYy1zdG9yZTANBgkqhkiG
9w0BAQsFAAOCAgEAnv1xAA+6Kft/4Kw/ZC+E0B5UbsJ/IdvlyFBTWw1B9nqj7zFs
ttwxGzVmY4j0GDfObm7JpdIAD2gm6nJm4lxUSJpnnbMbucN93dV7T6m+gt3IPqa2
qzqKZq/tgApm/92ldEB32GZofj3ise+EnA4luRkKJEVOwyPON1mzDpmK5AJEwCYM
iqo34DK3OupFubXnXHENJ2fBYqfHMjY3h2ATtXC116cuAb/S1Npl+mFL0HMdDN97
YN4afFqZD2U7pMKXklbW+dKPpU50TYz5SY/2NjTe81TYXslgaVj+YIlP1SEFoV7S
YhzhReh8Py9Ht10h9uDchWcq8P7XK1mgVKx/OlDFeyjvL1vd2pvUC/bcGSisZCJm
lwZE8oULoNoXWo4EDeywWOftSshgZIBQNqbkNQz67V97IyKBbhFr7Uo0kVPICopp
fhGKguQhqD4cYfCOSN0IsMJK7opi57eOcF8WCGsWRWyXJyNDRbet1iZ0v3u6OhlV
7xpeIVVdozHKssMOd8eTAoyfU985EYiNKW3+VVA7+iknRZXEwZTFTjAHLiqhYOBS
DHfZMuIyLIdGNnQHNxmUKQtPQwTh90rlKbhW8zWwxodxVcAHPyTyfvzfvk/FQ1vj
srZvygDvcOK6UJkrfq8TLAcK2CpeXZ+bE4enSM2pSLvu8WAGZOZ786SI1yc=
-----END CERTIFICATE-----
`)

func metricStoreCrtBytes() ([]byte, error) {
	return _metricStoreCrt, nil
}

func metricStoreCrt() (*asset, error) {
	bytes, err := metricStoreCrtBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metric-store.crt", size: 1545, mode: os.FileMode(0444), modTime: time.Unix(1646838230, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x92, 0x1e, 0xf4, 0xbc, 0x4a, 0x7f, 0xad, 0xb5, 0x9e, 0x8f, 0xc7, 0x2f, 0x13, 0xe2, 0x75, 0x80, 0x96, 0x8c, 0x54, 0xb6, 0xf4, 0x8a, 0x3e, 0x57, 0x67, 0xc4, 0xb5, 0xd3, 0x38, 0x47, 0xc7, 0x4c}}
	return a, nil
}

var _metricStoreCsr = []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIIChjCCAW4CAQAwFzEVMBMGA1UEAxMMbWV0cmljLXN0b3JlMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtqB1jT7jtkJ9cUZk2RBeAi8itAO3A087Gwf0
3wMw4BTRC0c86yi0dDe12sO8Lbm08DmUI3fUStsMttym65vGyUkEcdnNhYTvUlMb
W3bkjyvs+YZ+RPiKoF/McgGZTBGxZ3i/+wt4HA6eIhzBjMFvB6cE/dwV308xVswN
HcRkQOBj6n98lJUImUAmCLJ1I1WCZ8Kyu0E+ROuzQenM3zKm7dI4f0wpezwcJ/U/
wOCPnEzKH/fYoFBpvcAibTh00IGctLWOKgkdH1kRklfnzzSMein3LyaKy85p86UL
5IMEb2Ic0kAMhvNL/XRb1jF4x4pfzNVTflPukOx0bJkCn0v5+wIDAQABoCowKAYJ
KoZIhvcNAQkOMRswGTAXBgNVHREEEDAOggxtZXRyaWMtc3RvcmUwDQYJKoZIhvcN
AQELBQADggEBAIs4RZuJw0VLlxzCQLkQ0nkyQ9cRL5uhqWQBPR9qs9lTghfJeUhF
dLKDMRpmUZMGGG2sgBZev5wrjmMFdsBBytwzknbWbPy7+uDYSxaJkjq3uj/X7Dn0
f/iBqvcSN6LHmdKraVK2pSwHwSceuU9MHE8OssL1RdgzPDvTaNAQjKSp63FqfK7o
8R3BqRsJVXzVl5G+UwVJIdlBiBvKkUbx+zxtjDGEfAWY5qKeVCchuyq/ZssLZe05
XndWJ2hit6kFfa0ZdL3aVtChC5RyBpnt/mTvip4UY5k5ptW+RE7KNUQ0Yrfs4Zab
O30E1tiZcLg7r0ASfnin5VLCO+dsJATp7QI=
-----END CERTIFICATE REQUEST-----
`)

func metricStoreCsrBytes() ([]byte, error) {
	return _metricStoreCsr, nil
}

func metricStoreCsr() (*asset, error) {
	bytes, err := metricStoreCsrBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metric-store.csr", size: 952, mode: os.FileMode(0444), modTime: time.Unix(1646838229, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x7c, 0xe8, 0xa2, 0xa8, 0xdb, 0x0, 0xd, 0x77, 0x7b, 0xca, 0xf1, 0x8d, 0x8, 0x2a, 0x38, 0xcc, 0xbc, 0x5b, 0x5f, 0x41, 0x69, 0x4c, 0x8a, 0xfa, 0x57, 0x16, 0x9a, 0xc4, 0x54, 0x14, 0x41, 0xab}}
	return a, nil
}

var _metricStoreKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAtqB1jT7jtkJ9cUZk2RBeAi8itAO3A087Gwf03wMw4BTRC0c8
6yi0dDe12sO8Lbm08DmUI3fUStsMttym65vGyUkEcdnNhYTvUlMbW3bkjyvs+YZ+
RPiKoF/McgGZTBGxZ3i/+wt4HA6eIhzBjMFvB6cE/dwV308xVswNHcRkQOBj6n98
lJUImUAmCLJ1I1WCZ8Kyu0E+ROuzQenM3zKm7dI4f0wpezwcJ/U/wOCPnEzKH/fY
oFBpvcAibTh00IGctLWOKgkdH1kRklfnzzSMein3LyaKy85p86UL5IMEb2Ic0kAM
hvNL/XRb1jF4x4pfzNVTflPukOx0bJkCn0v5+wIDAQABAoIBAQCDrGWVFUbxXjc7
uNl1d8uQH5QR3qvRgwrGjpILSS2wItImI5LUqmCReqlvtbiz7zV6Dsm0WO2DmzQr
lCP1tDc7YZ4GyFbaceJrpOgQpkRcxfryXfokmF67CtdJS8XPhuI2DGW/B6Ht+Mwj
JECYz87R4aZDsq9CdsLIJg8+6x6tduOBt1NKClPFaoPwqdPq3rmJb5s5VwaOnYPz
fny6cIMf7mdqE5TCSllfp4CBoYJNweA7wr/o1ucyt6hfBEI6KWreo30D4WD0c4iP
KRDk8h0J+P/bgYHSxvTni93w2sGONR40nOjo4jGLIH0SbH94/PDX5u9/yjKVRSzX
PKBUy/MhAoGBAOf7I0n/crA1GCZSXTrUpdTLl3UymmkSan2OtBoRzlcmME0UC2lU
o++hIeGP439exD7qbOM+2qDJbKk/1xeInfBJm5II57hN/ivqPFyHX9Ite9ibRw8U
FfcziGXsSHPCjc1dATPEjSkdUiulCnMuiktKCPtOa7c4Rx8V5q0yRpaXAoGBAMmJ
JOjJBw03PdBzv/nlsolaf9y5YK8n3SVl39Cd3Eu0CJm8+tibcxMur05zuXM81sP0
7N68l9+R96h4fR5FHgy5Yt3QpySYudqnpMlV5p4EsxgHvZqvHOUugACB4SMqh0+Z
recNc1008WfZYE9QgilVDUYvdug6aoG3/dwPcag9AoGBAIPmUF9Pplc4KR4I8Md1
h0Ch7eEOP6uEdBYl4JN+ElOM/COnRQHDxV6HwKru1ExkhrK7OeRPpaGMRYNKMDNK
U3r/bzwuYgpyFhXEHkQCGOJ5SBSV3WZeZkri+yfwnBVtxpDA0+EqXZTF/iWgtntd
N/atBsRVB3vqvM2Y90r87hPLAoGAM1rbKOZxAZEeE0wrk0ZQ5GdHRbuHQ5ro42q+
Sa6wQCo0NtjNIv0Zqb2vtlIO46qRH4X+BhQQr0vGzAtH9rquGZfz9YoBzXWNhoZJ
m3RkO8f+yxTN3+jXeB8NRxPRhuCDcmk6wzHOP+YJzei6ffuJ73ZY15WouyyHj16P
NdBJqhECgYEAq0Q6grupGf3rMv719SoCKEPq8C+Mg7evsfWoqYWbdyvnZayTvJ93
Sw6FpTTfdz9t+C09WE0HshJpC+PhX4esfpAtGmsq8R9BLTZNKPx5DEV3PefSXI7m
zds78qgbkiMIBhzIYuVwpiUWSqVpkIJN2Gr9buqr8n3hqr8Zq1xNFfg=
-----END RSA PRIVATE KEY-----
`)

func metricStoreKeyBytes() ([]byte, error) {
	return _metricStoreKey, nil
}

func metricStoreKey() (*asset, error) {
	bytes, err := metricStoreKeyBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metric-store.key", size: 1679, mode: os.FileMode(0440), modTime: time.Unix(1646838229, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0xf1, 0x45, 0x2, 0x53, 0x58, 0x9, 0x7f, 0x9b, 0x1c, 0xcf, 0x7b, 0x7, 0x90, 0xe2, 0x20, 0xf, 0xef, 0x10, 0x35, 0x86, 0xe9, 0x35, 0x8a, 0x36, 0xe2, 0x90, 0xc2, 0x67, 0xfe, 0x93, 0xb6, 0x95}}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// AssetString returns the asset contents as a string (instead of a []byte).
func AssetString(name string) (string, error) {
	data, err := Asset(name)
	return string(data), err
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// MustAssetString is like AssetString but panics when Asset would return an
// error. It simplifies safe initialization of global variables.
func MustAssetString(name string) string {
	return string(MustAsset(name))
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetDigest returns the digest of the file with the given name. It returns an
// error if the asset could not be found or the digest could not be loaded.
func AssetDigest(name string) ([sha256.Size]byte, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return [sha256.Size]byte{}, fmt.Errorf("AssetDigest %s can't read by error: %v", name, err)
		}
		return a.digest, nil
	}
	return [sha256.Size]byte{}, fmt.Errorf("AssetDigest %s not found", name)
}

// Digests returns a map of all known files and their checksums.
func Digests() (map[string][sha256.Size]byte, error) {
	mp := make(map[string][sha256.Size]byte, len(_bindata))
	for name := range _bindata {
		a, err := _bindata[name]()
		if err != nil {
			return nil, err
		}
		mp[name] = a.digest
	}
	return mp, nil
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"metric-store-ca.crl": metricStoreCaCrl,
	"metric-store-ca.crt": metricStoreCaCrt,
	"metric-store-ca.key": metricStoreCaKey,
	"metric-store.crt":    metricStoreCrt,
	"metric-store.csr":    metricStoreCsr,
	"metric-store.key":    metricStoreKey,
}

// AssetDebug is true if the assets were built with the debug flag enabled.
const AssetDebug = false

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"},
// AssetDir("data/img") would return []string{"a.png", "b.png"},
// AssetDir("foo.txt") and AssetDir("notexist") would return an error, and
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		canonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(canonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"metric-store-ca.crl": {metricStoreCaCrl, map[string]*bintree{}},
	"metric-store-ca.crt": {metricStoreCaCrt, map[string]*bintree{}},
	"metric-store-ca.key": {metricStoreCaKey, map[string]*bintree{}},
	"metric-store.crt":    {metricStoreCrt, map[string]*bintree{}},
	"metric-store.csr":    {metricStoreCsr, map[string]*bintree{}},
	"metric-store.key":    {metricStoreKey, map[string]*bintree{}},
}}

// RestoreAsset restores an asset under the given directory.
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	return os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
}

// RestoreAssets restores an asset under the given directory recursively.
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(canonicalName, "/")...)...)
}
