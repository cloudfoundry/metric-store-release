package testing

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	. "github.com/onsi/gomega"
)

func NewServerRequest(method, uri string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, err
	}
	req.Host = req.URL.Host
	// set req.TLS if request is https
	if req.URL.Scheme == "https" {
		req.TLS = &tls.ConnectionState{}
	}
	// zero out fields that are not available to the server
	req.URL = &url.URL{
		Path:     req.URL.Path,
		RawQuery: req.URL.RawQuery,
	}
	return req, nil
}

func BuildRequest(method, url, remoteAddr, requestId, forwardedFor string) *http.Request {
	req, err := NewServerRequest(method, url, nil)
	Expect(err).ToNot(HaveOccurred())
	req.Header.Add("X-Vcap-Request-ID", requestId)
	req.Header.Add("X-Forwarded-For", forwardedFor)
	req.RemoteAddr = remoteAddr
	return req
}

func BuildExpectedLog(timestamp time.Time, requestId, method, path, sourceHost, sourcePort, dstHost, dstPort string) string {
	extensions := []string{
		fmt.Sprintf("rt=%d", transform.NanosecondsToMilliseconds(timestamp.UnixNano())),
		"cs1Label=userAuthenticationMechanism",
		"cs1=oauth-access-token",
		"cs2Label=vcapRequestId",
		"cs2=" + requestId,
		"request=" + path,
		"requestMethod=" + method,
		"src=" + sourceHost,
		"spt=" + sourcePort,
		"dst=" + dstHost,
		"dpt=" + dstPort,
	}
	fields := []string{
		"0",
		"cloud_foundry",
		"metric_store",
		"1.0",
		method + " " + path,
		method + " " + path,
		"0",
		strings.Join(extensions, " "),
	}
	return "CEF:" + strings.Join(fields, "|")
}
