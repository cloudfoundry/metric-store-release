package auth

import (
	"bytes"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
)

const requestIDHeader = "X-Vcap-Request-ID"

type templateContext struct {
	Timestamp       int64
	RequestID       string
	Path            string
	Method          string
	SourceHost      string
	SourcePort      string
	DestinationHost string
	DestinationPort string
}

type AccessLog struct {
	request   *http.Request
	timestamp time.Time
	host      string
	port      string
	template  *template.Template
	log       *logger.Logger
}

func NewAccessLog(req *http.Request, ts time.Time, host, port string, log *logger.Logger) *AccessLog {
	template, err := setupTemplate()
	if err != nil {
		log.Panic("error creating log access template", logger.Error(err))
	}

	return &AccessLog{
		request:   req,
		timestamp: ts,
		host:      host,
		port:      port,
		template:  template,
		log:       log,
	}
}

func (al *AccessLog) String() string {
	vcapRequestId := al.request.Header.Get(requestIDHeader)
	path := al.request.URL.Path
	if al.request.URL.RawQuery != "" {
		path = fmt.Sprintf("%s?%s", al.request.URL.Path, al.request.URL.RawQuery)
	}
	remoteHost, remotePort := al.extractRemoteInfo()

	context := templateContext{
		toMillis(al.timestamp),
		vcapRequestId,
		path,
		al.request.Method,
		remoteHost,
		remotePort,
		al.host,
		al.port,
	}
	var buf bytes.Buffer
	err := al.template.Execute(&buf, context)
	if err != nil {
		al.log.Error("Error executing security access log template", err)
		return ""
	}
	return buf.String()
}

func (al *AccessLog) extractRemoteInfo() (string, string) {
	remoteAddr := al.request.Header.Get("X-Forwarded-For")
	index := strings.Index(remoteAddr, ",")
	if index > -1 {
		remoteAddr = remoteAddr[:index]
	}

	if remoteAddr == "" {
		remoteAddr = al.request.RemoteAddr
	}

	if !strings.Contains(remoteAddr, ":") {
		remoteAddr += ":"
	}
	host, port, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		al.log.Error("Error splitting host and port for access log", err)
		return "", ""
	}
	return host, port
}

func toMillis(timestamp time.Time) int64 {
	return timestamp.UnixNano() / int64(time.Millisecond)
}

func setupTemplate() (*template.Template, error) {
	extensions := []string{
		"rt={{ .Timestamp }}",
		"cs1Label=userAuthenticationMechanism",
		"cs1=oauth-access-token",
		"cs2Label=vcapRequestId",
		"cs2={{ .RequestID }}",
		"request={{ .Path }}",
		"requestMethod={{ .Method }}",
		"src={{ .SourceHost }}",
		"spt={{ .SourcePort }}",
		"dst={{ .DestinationHost }}",
		"dpt={{ .DestinationPort }}",
	}
	fields := []string{
		"0",
		"cloud_foundry",
		"metric_store",
		"1.0",
		"{{ .Method }} {{ .Path }}",
		"{{ .Method }} {{ .Path }}",
		"0",
		strings.Join(extensions, " "),
	}
	templateSource := "CEF:" + strings.Join(fields, "|")
	return template.New("access_log").Parse(templateSource)
}
