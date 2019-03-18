package auth

import (
	"io"
	"net/http"
	"time"
)

type DefaultAccessLogger struct {
	writer io.Writer
}

func NewAccessLogger(writer io.Writer) *DefaultAccessLogger {
	return &DefaultAccessLogger{
		writer: writer,
	}
}

func (a *DefaultAccessLogger) LogAccess(req *http.Request, host, port string) error {
	al := NewAccessLog(req, time.Now(), host, port)
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
