package testing

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
)

//TODO rename file
type SpyUAA struct {
	secureConnection net.Listener
	tlsConfig        *tls.Config
	egressServer     *http.Server
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
	mux.HandleFunc("/oauth/token", oauthToken)
	s.egressServer = &http.Server{
		Handler: mux,
	}

	go s.egressServer.Serve(s.secureConnection)
	return nil
}

func (s *SpyUAA) Stop() {
	_ = s.egressServer.Close()
	_ = s.secureConnection.Close()
}

func tokenKeys(w http.ResponseWriter, _ *http.Request) {
	data, _ := json.Marshal(map[string]string{})
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func oauthToken(w http.ResponseWriter, _ *http.Request) {
	data := []byte(`{
					"access_token" : "some-token",
					"token_type" : "bearer",
					"id_token" : "eyJhbGciOiJIUzI1NiIsImprdSI6Imh0dHBzOi8vbG9jYWxob3N0OjgwODAvdWFhL3Rva2VuX2tleXMiLCJraWQiOiJsZWdhY3ktdG9rZW4ta2V5IiwidHlwIjoiSldUIn0.eyJzdWIiOiIwNzYzZTM2MS02ODUwLTQ3N2ItYjk1Ny1iMmExZjU3MjczMTQiLCJhdWQiOlsibG9naW4iXSwiaXNzIjoiaHR0cDovL2xvY2FsaG9zdDo4MDgwL3VhYS9vYXV0aC90b2tlbiIsImV4cCI6MTU1NzgzMDM4NSwiaWF0IjoxNTU3Nzg3MTg1LCJhenAiOiJsb2dpbiIsInNjb3BlIjpbIm9wZW5pZCJdLCJlbWFpbCI6IndyaHBONUB0ZXN0Lm9yZyIsInppZCI6InVhYSIsIm9yaWdpbiI6InVhYSIsImp0aSI6ImFjYjY4MDNhNDgxMTRkOWZiNDc2MWU0MDNjMTdmODEyIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImNsaWVudF9pZCI6ImxvZ2luIiwiY2lkIjoibG9naW4iLCJncmFudF90eXBlIjoiYXV0aG9yaXphdGlvbl9jb2RlIiwidXNlcl9uYW1lIjoid3JocE41QHRlc3Qub3JnIiwicmV2X3NpZyI6ImI3MjE5ZGYxIiwidXNlcl9pZCI6IjA3NjNlMzYxLTY4NTAtNDc3Yi1iOTU3LWIyYTFmNTcyNzMxNCIsImF1dGhfdGltZSI6MTU1Nzc4NzE4NX0.Fo8wZ_Zq9mwFks3LfXQ1PfJ4ugppjWvioZM6jSqAAQQ",
					"refresh_token" : "f59dcb5dcbca45f981f16ce519d61486-r",
					"expires_in" : 43199,
					"scope" : "",
					"jti" : "acb6803a48114d9fb4761e403c17f812"
				}`)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *SpyUAA) Url() string {
	_, port, _ := net.SplitHostPort(s.secureConnection.Addr().String())
	return fmt.Sprintf("https://localhost:%s", port)
}

func (spy *SpyUAA) Port() string {
	_, port, err := net.SplitHostPort(spy.secureConnection.Addr().String())
	if err != nil {
		panic(err)
	}
	return port
}
