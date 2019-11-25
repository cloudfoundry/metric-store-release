package testing

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
)

type SpyUAA struct {
	secureConnection net.Listener
	tlsConfig        *tls.Config
}

func NewSpyUAA(tlsConfig *tls.Config) *SpyUAA {
	return &SpyUAA{
		tlsConfig: tlsConfig,
	}
}

func (s *SpyUAA) Start() error {
	insecureConnection, err := net.Listen("tcp", ":0")
	if err != nil {
		return err
	}

	s.secureConnection = tls.NewListener(insecureConnection, s.tlsConfig)
	mux := http.NewServeMux()
	mux.HandleFunc("/token_keys", tokenKeys)
	egressServer := &http.Server{
		Handler: mux,
	}

	go egressServer.Serve(s.secureConnection)

	return nil
}

func tokenKeys(w http.ResponseWriter, _ *http.Request) {
	data, _ := json.Marshal(map[string]string{})
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *SpyUAA) Url() string {
	_, port, _ := net.SplitHostPort(s.secureConnection.Addr().String())
	return fmt.Sprintf("https://localhost:%s", port)
}
