package testing

import (
	"crypto/tls"
	"net"
	"net/http"

	mux2 "github.com/gorilla/mux"
)

type K8sSpy struct {
	secureConnection net.Listener
	tlsConfig        *tls.Config
	server           *http.Server
}

func NewK8sSpy(tlsConfig *tls.Config) *K8sSpy {
	return &K8sSpy{
		tlsConfig: tlsConfig,
	}
}

func (spy *K8sSpy) Start() error {
	insecureConnection, err := net.Listen("tcp", ":0")
	if err != nil {
		return err
	}

	spy.secureConnection = tls.NewListener(insecureConnection, spy.tlsConfig)
	mux := mux2.NewRouter()
	mux.HandleFunc("/apis/certificates.k8s.io/v1beta1/certificatesigningrequests", spy.csrs)
	mux.HandleFunc("/apis/certificates.k8s.io/v1beta1/certificatesigningrequests/metricstore/approval", spy.approveCsr)
	mux.PathPrefix("/apis/certificates.k8s.io/v1beta1/certificatesigningrequests/metricstore").Methods("GET").HandlerFunc(spy.getCsr)
	mux.PathPrefix("/apis/certificates.k8s.io/v1beta1/certificatesigningrequests/metricstore").Methods("DELETE").HandlerFunc(spy.deleteCsr)
	mux.PathPrefix("/metrics").Methods("GET").HandlerFunc(spy.metrics)
	mux.PathPrefix("/").HandlerFunc(spy.everything)
	spy.server = &http.Server{
		Handler: mux,
	}

	go spy.server.Serve(spy.secureConnection)
	return nil
}

func (spy *K8sSpy) everything(_ http.ResponseWriter, r *http.Request) {
	panic("got uncaught path: " + r.RequestURI)
}

func (spy *K8sSpy) csrs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(csrRequestBody(false))

}

func (spy *K8sSpy) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(200)
	w.Write(csrRequestBody(false))

}

func csrRequestBody(includeCertificate bool) []byte {
	certificate := ""
	if includeCertificate {
		certificate = `,    "certificate": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUVORENDQXh5Z0F3SUJBZ0lVYlBkTjFIVjhGNHprK1FwYS9xSDBQQ1BJTTZjd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0RURUxNQWtHQTFVRUF4TUNZMkV3SGhjTk1Ua3hNREE1TVRZd01qQXdXaGNOTWpBeE1EQTRNVFl3TWpBdwpXakFrTVJBd0RnWURWUVFLRXdkbWIyOHRZbUZ5TVJBd0RnWURWUVFERXdkbWIyOHRZbUZ5TUlJQ0lqQU5CZ2txCmhraUc5dzBCQVFFRkFBT0NBZzhBTUlJQ0NnS0NBZ0VBMjBiWFk4MUorbVZIa0tuYnpDYzhRbTU5MXFiUG9lSTcKT3VHTndIZXRZNFNCTjZQUytMWHJwSVpZME1MR1dCa3c0cXYxVWd1Z05tRFJPY0J3b0lHdEM5Tm0zTjB5L01CMQpxSDFPSFlidndGZHQ2b0pqcHFxeS9lcXNwdExSY0V4OGxIdVRUckhyQkNVRVpINnRwQXFoNUFOUHVWYk1XMUdQCkJRQitjNEZTVjRkaXE4ZUVyb1JnbEJrNzYvdjN6MDJBV1ZHSU9idmlxSFRMNGtBVjh1VnRzNGJXcGxHbTZaWWMKY0MwTFFUL0VsZmd3UnNzaW1uaEdOUVJ4UnA0cU14M1pHK1V3YXJURGNxRUdmZFJnOWVYd2JrQTJuc3JiV0t3YgpxbUp0M3FJTnA2NGR4U2dTWExhcUhMWnN0SWh5QkVnWmJMWWxqSkVSNHBhWFR5VVU3TTIrck1MRUhDWlIzcjNkCmxydTNQTmJSTndEazd2Y1Q5bGYzMXhKV0hvSnlFVm85L2U5c3F6a2dOVTZrOUw5Wktsc2NwYVNya2tUME5WczQKV2hUYWZzSkFZYzMvUVVSZ0RmdDRzdFBEdjlXdEhLSjRBME0xelZnak1teEhsRVFqaU5VTkhpdmdmN1BFSUY2MwpCd2RFZFc2K2xxUXg3RWVnTGFRcEExaVFUQ01pUlYzT2NCZi9YczJKRUkvK2U5SWhScUdhVzNMTnk0MjF3M0psCjFhQyt6QklRL3d2L1hwUkRwNU9aeDdTRldFaXBLSnNnRmFFRnA3R1lVZDd5N1llZlNHSW91b3JlRExhZURwUnkKS1ZGY0ZaQ29YcnJuczdTcjRpVWdUeTBIQUxremRyK2xTQzJXWWZtR2x3VEJrOGd2WjFYdzgvbHBoRXltVWhvLwpaQlRTaXJ0T2hDc0NBd0VBQWFOMU1ITXdEZ1lEVlIwUEFRSC9CQVFEQWdXZ01CTUdBMVVkSlFRTU1Bb0dDQ3NHCkFRVUZCd01DTUF3R0ExVWRFd0VCL3dRQ01BQXdIUVlEVlIwT0JCWUVGQU90TzUyanVySWc2K05qYk5wcWxnWXcKbDd4WU1COEdBMVVkSXdRWU1CYUFGQjJ3OUQ4NVZZMFlEVnVwa2dibWRyMVZIdVJRTUEwR0NTcUdTSWIzRFFFQgpDd1VBQTRJQkFRQStkL0QyQWhyb1NmRWQra3NCenFkWk1OZDc5YnZuWGp6QS8yNjhPNlE4blpienNndWszeVdjCkdndkQ4VlNPSEZOYkRZRy9DeEM0WW5zSk1MTjBpdkxscFRyRE5wTHZQK2kvamNEUk12OER1N1dzRnJ1ODErWi8KbkgzbE5JYm1WWmlYRGZlcWYzK0RTdDg2Y1RRNTJkbElWVkZrNHl6NWU2ajFOYzJiclJYQXVOTGs3c2liZkJjUAp1NEVsbWJYNitFRTBkb01TVzVWb3Bhczhoa01LZGdBMFREdEFtcjJlK3lMYlg2YndDUE5HZUhZcy9vWVF3SkdlCmF6QUc0ZkRsZDBpVWR5b0YzMzZLOFc4NTFqcExTUGE2TmZrem1vbFlPbVJZYis4UEZJVFpSdUFteWZyV3B0WnUKblFxZi9BVHN4OGdlR1RHWGptT2phdzd0REM4ZnZVd0gKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo="`
	}
	return []byte(`{
  "metadata": {
    "name": "metricstore"
  },
  "spec": {
    "groups": [
      "system:authenticated",
      "$name"
    ],
    "request": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJRWFUQ0NBbEVDQVFBd0pERVFNQTRHQTFVRUNoTUhabTl2TFdKaGNqRVFNQTRHQTFVRUF4TUhabTl2TFdKaApjakNDQWlJd0RRWUpLb1pJaHZjTkFRRUJCUUFEZ2dJUEFEQ0NBZ29DZ2dJQkFOdEcxMlBOU2ZwbFI1Q3AyOHduClBFSnVmZGFtejZIaU96cmhqY0IzcldPRWdUZWowdmkxNjZTR1dOREN4bGdaTU9LcjlWSUxvRFpnMFRuQWNLQ0IKclF2VFp0emRNdnpBZGFoOVRoMkc3OEJYYmVxQ1k2YXFzdjNxcktiUzBYQk1mSlI3azA2eDZ3UWxCR1IrcmFRSwpvZVFEVDdsV3pGdFJqd1VBZm5PQlVsZUhZcXZIaEs2RVlKUVpPK3Y3OTg5TmdGbFJpRG03NHFoMHkrSkFGZkxsCmJiT0cxcVpScHVtV0hIQXRDMEUveEpYNE1FYkxJcHA0UmpVRWNVYWVLak1kMlJ2bE1HcTB3M0toQm4zVVlQWGwKOEc1QU5wN0syMWlzRzZwaWJkNmlEYWV1SGNVb0VseTJxaHkyYkxTSWNnUklHV3kySll5UkVlS1dsMDhsRk96Tgp2cXpDeEJ3bVVkNjkzWmE3dHp6VzBUY0E1TzczRS9aWDk5Y1NWaDZDY2hGYVBmM3ZiS3M1SURWT3BQUy9XU3BiCkhLV2txNUpFOURWYk9Gb1UybjdDUUdITi8wRkVZQTM3ZUxMVHc3L1ZyUnlpZUFORE5jMVlJekpzUjVSRUk0alYKRFI0cjRIK3p4Q0JldHdjSFJIVnV2cGFrTWV4SG9DMmtLUU5Za0V3aklrVmR6bkFYLzE3TmlSQ1AvbnZTSVVhaAptbHR5emN1TnRjTnlaZFdndnN3U0VQOEwvMTZVUTZlVG1jZTBoVmhJcVNpYklCV2hCYWV4bUZIZTh1MkhuMGhpCktMcUszZ3kybmc2VWNpbFJYQldRcUY2NjU3TzBxK0lsSUU4dEJ3QzVNM2EvcFVndGxtSDVocGNFd1pQSUwyZFYKOFBQNWFZUk1wbElhUDJRVTBvcTdUb1FyQWdNQkFBR2dBREFOQmdrcWhraUc5dzBCQVEwRkFBT0NBZ0VBMFMwVQp2a25OSEREcTZzVXVNY3lkUDNkOTJRUVl1QStoNmc5bDVSZG1Dam54bXI5NEsrVmZZcUV5dEJxWkZNQXl1Y3hKCmc5UGRTOFNHLzloYnFydTVHT0dYU1ErZlpWZkd3VGwzOUgxY3ArWW02Nm9Ycm9iYUMvTytySlRBR1llTkNrTGcKZzdYa3kwT0dXOTlFRVFsNnZIOEtGck5iNW5VTm1pWTFiK0hKVnRCdzIzZUxLRXFTMHZvTmRpTUhlQVljTThQdgpDRmNLTUdlaHQvVlRlNS9RTjJxYjBxR2ZHeFBMRzdYVklGMEw4OW1QYWx5ZFNrQ2FvVTI1VDZMb1dFTmNKK1NXClNSV1I2Uk0xL2RXeTgySFJXVk1IVmxVUVBQYmh3Z1lqYnlhR2xVSnVURktQOWNVbUlGWXhYSGo4bkYzSkVnT2gKUHJadW1mQ0pHZEcxVzYzUjBRcXFFcU5SWW0wTm9HaldVbDlaOThNTVpVRktaM09adHgxSGxqcXh4dUQ2eDVtcQpjeGh3UC9DWHdBanFvckY4cG1BQ1R0TzY0RHMwODhSbSt2K3c4MlNiZ0srd0hmU1ludGk2UU1nY2lidkdmTXFjCkxOUmhqZXZaaHFydjZSeDQvbGFRaG5oMG5BaS9scjdpSDJUbG5RS2Joc3Z4MURNZ3MvUGdUUmYvQjVlZ2ppZ0QKWkJPcmVKQ2l0c1pvU0pjc2lHaFhVZ0hTNmlTTjhTKzkzT0h4QnZkZFI0OElFSTJiUlozWjJHU0RuS0J5dkNndApiRW90dTVtcmtNbldNYTd3ZVlGNzIyMzlieFFHN2NNdXp2TnpJZ0wrRGNQbjl6aTUyMjg4WFl6RnkycmNUNlNsCjF5OVpwU0pDR0I0clZad1lOMzVoQ3JqbmZMeCt4L2tKSmdPN3JNcz0KLS0tLS1FTkQgQ0VSVElGSUNBVEUgUkVRVUVTVC0tLS0tCg==",
    "usages": [
      "client auth",
      "digital signature",
      "key encipherment"
    ]
  },
  "status": {
    "conditions": [
      {
        "message": "approving it",
        "reason": "because I can",
        "type": "Approved"
      }
    ]` + certificate + `
  }
}`)

}

func (spy *K8sSpy) approveCsr(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(csrRequestBody(false))
}

func (spy *K8sSpy) getCsr(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(csrRequestBody(true))
}

func (spy *K8sSpy) deleteCsr(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(202)
	w.Write([]byte{})
}

func (spy *K8sSpy) Stop() {
	spy.server.Close()
}

func (spy *K8sSpy) Addr() string {
	return spy.server.Addr
}

func (spy *K8sSpy) ConnectionInfo() (string, string) {
	_, port, err := net.SplitHostPort(spy.secureConnection.Addr().String())
	if err != nil {
		panic(err)
	}
	return "localhost", port
}

func (spy *K8sSpy) Port() string {
	_, port, err := net.SplitHostPort(spy.secureConnection.Addr().String())
	if err != nil {
		panic(err)
	}
	return port
}
