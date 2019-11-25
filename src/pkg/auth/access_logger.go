package auth

import (
	"io"
	"net/http"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/logger"
)

type DefaultAccessLogger struct {
	writer io.Writer
	log    *logger.Logger
}

func NewAccessLogger(writer io.Writer, log *logger.Logger) *DefaultAccessLogger {
	return &DefaultAccessLogger{
		writer: writer,
		log:    log,
	}
}

func (a *DefaultAccessLogger) LogAccess(req *http.Request, host, port string) error {
	al := NewAccessLog(req, time.Now(), host, port, a.log)
	_, err := a.writer.Write([]byte(al.String() + "\n"))
	return err
}

type NullAccessLogger struct {
}

func NewNullAccessLogger() *NullAccessLogger {
	return &NullAccessLogger{}
}

func (a *NullAccessLogger) LogAccess(req *http.Request, host, port string) error {
	return nil
}
