// Code generated for package testing by go-bindata DO NOT EDIT. (@generated)
// sources:
// certs/metric-store-ca.crl
// certs/metric-store-ca.crt
// certs/metric-store-ca.key
// certs/metric-store.crt
// certs/metric-store.csr
// certs/metric-store.key
package testing

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)
type asset struct {
	bytes []byte
	info  os.FileInfo
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
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _metricStoreCaCrl = []byte(`-----BEGIN X509 CRL-----
MIICiTBzAgEBMA0GCSqGSIb3DQEBCwUAMBoxGDAWBgNVBAMTD21ldHJpYy1zdG9y
ZS1jYRcNMjAwOTA4MjAyNjA1WhcNMjIwMzA4MjAyNjAyWjAAoCMwITAfBgNVHSME
GDAWgBT126ddSwWHm+HhepXxGtiozSaeWzANBgkqhkiG9w0BAQsFAAOCAgEAwaiY
OL/zBQMrp/+6sZbK80J7OzUtIFROYScRlaqPVgfd/T68ilUbktaavCJiBkp7D06d
zlc9Br8sC8A0UdcgnJp32VJ6oW7FYFcOL4Z7nMNWtJWyNZ1W/ssY/mGhI63isI3A
w7RaKKrkPAP8Gbwg8aECsUaOSH9oNzwkC9ypMrjmN/kncuBVhr9vpnaXB234fojC
zL6aIHMzkf79M7QhJ157xXIZh26kk68GNZVkNJnm/4c8u8R+dDU3YdFwbrYDL+Na
PGdDK+B22G77u13VdQ7wpllrfB31nQNQB4wT9N81xJqD10JGzUgNwTj/jQwAG/+X
C40aBVuSiaQmMINS36I+yb7tubrt4WIajeqtP8QCfSli+o8hlfAvklzd08F+T3xG
hXi4IB51lHxJ4kD78UMeZQwWWn21Dp6RvBRpFFPY06A1E6gyKFyGy3qdUTfstBmM
HQYAQ3Xa0aPxJgeTw4STOWXkorYrloRW7qq+orvq0bCJZDuXnH2obWJPJLIdTAoi
v1PlwYkMtnNtpxDJTZwZi+8yWQYn98EP5BeUAl4F/eCs/LDa/qWdTWhmXOg3W1u0
hmoUegnM+zIZZ9Ja1KN7NNWjyZqcdGnxknwWjKmJ8KxvH1xqb4iIDAuDTgSdkWbI
YQzkdXepkm87JwvRQP+88a6cQLljWZ/vF7upCMU=
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

	info := bindataFileInfo{name: "metric-store-ca.crl", size: 934, mode: os.FileMode(292), modTime: time.Unix(1599596765, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _metricStoreCaCrt = []byte(`-----BEGIN CERTIFICATE-----
MIIE9DCCAtygAwIBAgIBATANBgkqhkiG9w0BAQsFADAaMRgwFgYDVQQDEw9tZXRy
aWMtc3RvcmUtY2EwHhcNMjAwOTA4MjAyNjA1WhcNMjIwMzA4MjAyNjAyWjAaMRgw
FgYDVQQDEw9tZXRyaWMtc3RvcmUtY2EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAw
ggIKAoICAQDCxkCNiEewuRl9GdLJCUix3rWhiWGMsUf0pkPdP0DF/jkeWRsTBomy
c+lyiqTYuHtYwmRe2F7qSBQ0t33DvTdwgKYTbmwpqghwLPi/2eWN02pISsLO1Ike
HbM7TLxcVwVM2xCIqQZezvdA+erJay2logEGZlil/j6ckdjpZm5rlk6nKTXAQGf9
dIU9Xy5iDtjz0fI+LnwPDi6S9PsGKgswCLshBBVVUXN/L20X/B/bEv62SVBNnd3m
oWdRaEWcs8GU70u53owel6uDxXstc6OMQafCyFAyqkuf/5SIDexa0ykBQ1ArEL09
sVLOIRvhBy+/98LIGEmtQ0OEhYcu+2GGA/WDDXexkiT20t4SvjMhmd3OTQTsFT5g
f6K0nSW4DwyDonQIj/HF5PFagjzVDkMQaegfDpNVx+5uU1V//CCKqGpEtqNvzw4f
7Go946KWUnweUer8bCT69lDCs9O3XEspFVSz0kNW0+ZJltnE7Sq7e8abpisPi27S
Ro546avKK+OjSuZ5RQ9/EyrP1dXCnukGQRhTgt/cVPiMQfO1/sZYmrJDQ2BHIuCv
4dYz8rr+7W2TulAWc83YYr3ObITt0aU0fXalETBBlU1bFmlcVp6ei8uXS9ITb8wa
9RgIGIWf2vHcTfxpto6yqpMyEUj9Y+Tyb/ZM84gXK90FVDYGRNy5KwIDAQABo0Uw
QzAOBgNVHQ8BAf8EBAMCAQYwEgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQU
9dunXUsFh5vh4XqV8RrYqM0mnlswDQYJKoZIhvcNAQELBQADggIBADVvAV5XppX/
BX3QhDJZzsmlyeennjXQEPER2EgZaqId0DwQbSkNCSFh1Y71bhHDL5bl0GQwS35r
K8xVhPFqIooJE/MdPAwbjJPHEovDo7gXq0QV4TjdJLQt+PFSZ3v0J0Qse4T+G/aA
WhgjMYB7gZNyI17vJLQjHQSDBd0BgkJuRgRVqMWhph2//RxDZbFVZgabS0l6acqt
rJxBZ/e2S+6rCe16x/8/S2/V6MuCIqnLGUfPH1vPdBPgpBoTvf6N3UJQXeIpiHBA
CAN4iqD73nxjv1oX0mgNC/S60ZVZ1a4Njb6lCHnPSYJ9hF3PtW9MpMXdBBHsb5GB
yFkz13QeP9RFoudfWzK8R2pcTodQ2NcPVF3rXO5/JTBugOAO3D4bLlQsn1BNUyqX
L6PpNUakJhiB7i5CCm/EJ1mCT7mR5Y6JxR7Ur8UAXy5AWIQi4UYsJnobPHQBe/O2
2J24sc+Y+D61jUV7kbwikmdd3dB/sdxPn4zCf63tdGY8mq+NMyIMvoFadiIvXiRo
3pcU42s42O2e03XFZPWTZbU/FP5SOjtGgRzztI+AHAG8J4mF2Bf83xpHe60ETejK
HLm5HT39rZ6eEHpql3rHTzh0OdXyYabmxjNaINYhyuLh6L/lycWPRQeau+o/IY+a
TbEx3cMrt3+O5OPPZHfndl4YeP77t61V
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

	info := bindataFileInfo{name: "metric-store-ca.crt", size: 1777, mode: os.FileMode(292), modTime: time.Unix(1599596765, 0)}
	a := &asset{bytes: bytes, info: info}
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

	info := bindataFileInfo{name: "metric-store-ca.key", size: 3247, mode: os.FileMode(288), modTime: time.Unix(1599596765, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _metricStoreCrt = []byte(`-----BEGIN CERTIFICATE-----
MIIESDCCAjCgAwIBAgIRAJLLrSQbFLCIF+GPJD4wMc4wDQYJKoZIhvcNAQELBQAw
GjEYMBYGA1UEAxMPbWV0cmljLXN0b3JlLWNhMB4XDTIwMDkwODIwMjYwNloXDTIy
MDMwODIwMjYwMVowFzEVMBMGA1UEAxMMbWV0cmljLXN0b3JlMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEAu8V4Mlwjpc9j7U9uoL7sgrpNk2eH+Jk5zgVm
qQC1od3//QZAs4ca6R+NDkdgT9xX72HmxrltqomzjBf1tPOR+ElUtXLq0UNNN9kI
A+XFGq+mOMVVYlEYS5e+odydvMBaBiMVtKK9s7F+xWtt7y1Frr0I+5np3Gy4EErD
G1h2Dmol9wCeBrdtodR8TN0HRPXDertmUfQYYlG/jgd/b+SCj/9wM0PwE8KkVTyw
lLkLpjIgTlt5MUPjGwqRbLAoxRvZEJSncAWQpdw/aO0g69dqx35FXzKzNHfZiTaT
BmjPDecrfgNi55N9O6DEwrQyqnBsOFcTelB6gz9hoZXCOoUN2QIDAQABo4GLMIGI
MA4GA1UdDwEB/wQEAwIDuDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIw
HQYDVR0OBBYEFNUusIDGeOmK/XPwSKZuFGfCvR7BMB8GA1UdIwQYMBaAFPXbp11L
BYeb4eF6lfEa2KjNJp5bMBcGA1UdEQQQMA6CDG1ldHJpYy1zdG9yZTANBgkqhkiG
9w0BAQsFAAOCAgEAB4fbTyDSrEuszNmuJRgdH6iEG9CSdJASSHP5necSFXMpY+67
bA3zD8H2gJ99EtQRzYUg+x+rBLUlqssyn7/2Wlh+Pr1/CSrh4w9DDtbNx1Z5Ki65
DjC2lLJChKmLVo1WHDx0qRGRHOAHkrnbF99T3ipgfJSG0SwZvLYUOi+DtyyeE+lL
g1VfkCUgdIpCa77AoVpkzwp4s9V5EHFjC2+zJdkCG7iS0FRVXZTcqdCIQqQDwM5I
URTsKuawXeDYrDIubVwbN0MTnVO2/frQ5ERHMDImp2+LS7qLN58t/pJl3qqF4Qx5
WnVsJqK73iJSGRcg1nu4frActAY8Mkc+VFbeqLk2/NA3AURxK65osiIR4/oKVOyJ
Gqx8PLAVwX0zLyaS5xDG0URyiTI2R2fvmaxfk+WuwVjRGbbYT9eDewwMTpm+4NKV
h5j15H1rUlNfCysmH9OjkSPmAX2dtw5l7Sb31nSMbT7n4CSzx83fV1SkZ87n8oVY
wyCdusNKLsC5HWfx47cjZ2Rl7kCzq6tsLFC45daKMfxrrXeRYjLcoWEkvXhDSUx/
QIblqxkmGl7r4oPR8aXx7BjO9RETcy45UQyOe3URPURvmfjecNbuigdYbRaF5Mjb
TmYLl4CtHXO2RCAPdY/jOYc2L0wbHaPqrLKeNijVPfDSD9yOMeQAG69sM9U=
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

	info := bindataFileInfo{name: "metric-store.crt", size: 1545, mode: os.FileMode(292), modTime: time.Unix(1599596766, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _metricStoreCsr = []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIIChjCCAW4CAQAwFzEVMBMGA1UEAxMMbWV0cmljLXN0b3JlMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEAu8V4Mlwjpc9j7U9uoL7sgrpNk2eH+Jk5zgVm
qQC1od3//QZAs4ca6R+NDkdgT9xX72HmxrltqomzjBf1tPOR+ElUtXLq0UNNN9kI
A+XFGq+mOMVVYlEYS5e+odydvMBaBiMVtKK9s7F+xWtt7y1Frr0I+5np3Gy4EErD
G1h2Dmol9wCeBrdtodR8TN0HRPXDertmUfQYYlG/jgd/b+SCj/9wM0PwE8KkVTyw
lLkLpjIgTlt5MUPjGwqRbLAoxRvZEJSncAWQpdw/aO0g69dqx35FXzKzNHfZiTaT
BmjPDecrfgNi55N9O6DEwrQyqnBsOFcTelB6gz9hoZXCOoUN2QIDAQABoCowKAYJ
KoZIhvcNAQkOMRswGTAXBgNVHREEEDAOggxtZXRyaWMtc3RvcmUwDQYJKoZIhvcN
AQELBQADggEBABxWn76zIGYqYhL/4Zu3n5T/Fu5V9VIKdst1GdBfn7gvGmxzLHNp
oL+RlrR6yDjdeq9xePJBMZVEMECnYrvzKCTPNllhVYjisMM19z8KsxwyB+Kc9rGw
m+oHRsaXoq81UH71KwXDmuVdVGj9vi8CkrcM8/4Tl4bqHR6DBrBL+TLfleWYwG7k
cLj3PfOgEvXFsCB6ScFEDvZ2nB8autyI5kI1rjXR/2DxUAgCdBLYn+3tokveojvr
DcMn+9PqVohG27eEX8yF8xplJ466Naj9T4z4WL/gxh/7jAphJu2Bag9w/htBTVUK
qTvrm0Izf9xqQQLjy1M2e3aq4iQ1Xgup5C0=
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

	info := bindataFileInfo{name: "metric-store.csr", size: 952, mode: os.FileMode(292), modTime: time.Unix(1599596766, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _metricStoreKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAu8V4Mlwjpc9j7U9uoL7sgrpNk2eH+Jk5zgVmqQC1od3//QZA
s4ca6R+NDkdgT9xX72HmxrltqomzjBf1tPOR+ElUtXLq0UNNN9kIA+XFGq+mOMVV
YlEYS5e+odydvMBaBiMVtKK9s7F+xWtt7y1Frr0I+5np3Gy4EErDG1h2Dmol9wCe
BrdtodR8TN0HRPXDertmUfQYYlG/jgd/b+SCj/9wM0PwE8KkVTywlLkLpjIgTlt5
MUPjGwqRbLAoxRvZEJSncAWQpdw/aO0g69dqx35FXzKzNHfZiTaTBmjPDecrfgNi
55N9O6DEwrQyqnBsOFcTelB6gz9hoZXCOoUN2QIDAQABAoIBAQCh9OYCkeSRbLsl
AgFKlsMK0sR8oqzt6MOqBpCQrrL7Ra85v73o21yDvRn+OeRBna0fJZNWzrNfh9wc
tsHQbNH3lNCCnPcavfEJfaHjMrj6loxJpTNLVOUetmjP1akcF9DOQE7FeiUjq7HL
eCjfRm43FId99Dh5TjDIpKN6n6dcMAnrFMhbdU7rD5VD1W7gdTgPHFNCiCGVom+H
PJOujkOysNbuBJFM46uT3luZZ3akowfhoxI0lDWqqig86tdxexQhAMzTQPC9I/cP
08/W6DOb00dxA5PIYHjzsjK1mMZsF4TfrQuwYGkslKrvypxFQedwTtMeh1D0CHsz
H/OAnLaBAoGBAOErHQSJoeC8qklmj6iC1cmq0aytLDW7cjI7i6ni7Asfm4n8kGbr
+KIvfxjJrpCAO7bLW2ZNKmVWCch+XbvmT/b71eZ9jN9y1F6rqnyQB9S1UCC6Byh7
gMcYsv6gVRcirI/jC3uBDmPFFeliokUW6vqKDvXMUAU9Z6qEkCufUQbxAoGBANV7
d8MZJNjT+vLCv3c7g8G51CqWmpZyVsbYFnBlA1yZ/CMMwNONeX47/tZeZ1EgAQ1x
SdAFK+1NFLwXaC8/9tTjY/cm+HDc2nib/Nvu1FWoWyddnWR9krrxMLt6kCuGwMdL
PAnG2OS+RBCVCVh9C0NAHytGCVY/t7x+0PXwjoVpAoGBAKwiXnOamAMLmA629jn3
k2IxUUt1s6d8Hgfi15lPXe3/AtQRHX9hA9lRABO+EtJrBbtvaPcjJLcFeEMqv5Om
tRj2WwZykqA707hv+cxx+1qUJaZvMIu1JrSN4ECh54rhOhRhmOSYu5xwDZk2iyDQ
LWDM7DTiNYZb9AU6hFCk4bexAoGAQGtBeF3eAI/26cpafGA5IfwxSaiofT2Dcf1C
yCezG/5bVzhB95R5VN5Fx+o0wwYlSykkXOEyoCjiWN+3UIq8sQDs6WeZEHWUd1Ca
vMMUz8Q9vWNCW1CJNmARlIEnf/rpsTnCpDCcwmmnoFlYuJsDCwgOX8CCkMQpbXfX
Fl/AogECgYBNWmmm2wXYk58Dwop/6ZIP/VjMJayXMFhQdI+USd73lFL9WljrMoME
lKWsFh6//5qNB31I3a+Y9WweG7Fo6X5vKvgMvgfTfFLjsIzdzFl3eP9dLBS84SnC
m/DigDsDv07xx0Cvpg8aIDYcB1LBdJUNuAYch6aHy7NXOydpnropFg==
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

	info := bindataFileInfo{name: "metric-store.key", size: 1679, mode: os.FileMode(288), modTime: time.Unix(1599596766, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
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

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
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

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
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
	"metric-store-ca.crl": &bintree{metricStoreCaCrl, map[string]*bintree{}},
	"metric-store-ca.crt": &bintree{metricStoreCaCrt, map[string]*bintree{}},
	"metric-store-ca.key": &bintree{metricStoreCaKey, map[string]*bintree{}},
	"metric-store.crt":    &bintree{metricStoreCrt, map[string]*bintree{}},
	"metric-store.csr":    &bintree{metricStoreCsr, map[string]*bintree{}},
	"metric-store.key":    &bintree{metricStoreKey, map[string]*bintree{}},
}}

// RestoreAsset restores an asset under the given directory
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
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
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
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
