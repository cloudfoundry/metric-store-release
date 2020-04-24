package testing

import (
	"crypto/tls"
	"encoding/base64"
	"net"
	"net/http"

	uuid2 "github.com/google/uuid"
)

type PKSSpy struct {
	k8sApiHost       string
	k8sApiPort       string
	k8sCaCertificate string
	secureConnection net.Listener
	tlsConfig        *tls.Config
	server           *http.Server
}

func NewPKSSpy(k8sApiHost, k8sApiPort string, k8sCaCertificate string, tlsConfig *tls.Config) *PKSSpy {
	return &PKSSpy{
		k8sApiHost:       k8sApiHost,
		k8sApiPort:       k8sApiPort,
		k8sCaCertificate: k8sCaCertificate,
		tlsConfig:        tlsConfig,
	}
}

func (spy *PKSSpy) Start() error {
	insecureConnection, err := net.Listen("tcp", ":0")
	if err != nil {
		return err
	}

	spy.secureConnection = tls.NewListener(insecureConnection, spy.tlsConfig)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/clusters", spy.clusters)
	mux.HandleFunc("/v1/clusters/cluster1/binds", spy.clusterBinds)
	spy.server = &http.Server{
		Handler: mux,
	}

	go spy.server.Serve(spy.secureConnection)
	return nil
}

func (spy *PKSSpy) clusters(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	data := []byte(`[
  {
    "name": "cluster1",
    "plan_name": "small",
    "last_action": "CREATE",
    "last_action_state": "succeeded",
    "last_action_description": "Instance provisioning completed",
    "uuid": "021e49a0-f59a-4dd2-b163-2a71f98d5286",
    "kubernetes_master_ips": [
      "localhost"
    ],
    "parameters": {
      "kubernetes_master_host": "` + spy.k8sApiHost + `",
      "kubernetes_master_port": ` + spy.k8sApiPort + `,
      "kubernetes_worker_instances": 3
    }
  }
]`)
	w.Header().Set("Content-Type", "application/vnd.spring-boot.actuator.v2+json;charset=UTF-8")
	w.WriteHeader(200)
	w.Write(data)
}

func (spy *PKSSpy) clusterBinds(w http.ResponseWriter, r *http.Request) {
	uuid, err := uuid2.NewRandom()
	if err != nil {
		panic(err)
	}

	if r.Header.Get("Authorization") == "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	encodedCaCert := base64.StdEncoding.EncodeToString([]byte(spy.k8sCaCertificate))
	println("return cluster binds host: ", spy.k8sApiHost)
	data := []byte(`{
  "preferences": {},
  "apiVersion": "v1",
  "kind": "Config",
  "current-context": "cluster1",
  "contexts": [
    {
      "context": {
        "cluster": "cluster1",
        "user": "8be333bf-50af-43b0-a344-b28c2cdaf63a"
      },
      "name": "cluster1"
    }
  ],
  "clusters": [
    {
      "name": "cluster1",
      "cluster": {
        "certificate-authority-data": "` + encodedCaCert + `",
		"server": "https://` + spy.k8sApiHost + ":" + spy.k8sApiPort + `"
      }
    }
  ],
  "users": [
    {
      "name": "` + uuid.String() + `",
      "user": {
        "token": "some-token-k8s"
      }
    }
  ]
}`)
	w.Header().Set("Content-Type", "application/vnd.spring-boot.actuator.v2+json;charset=UTF-8")
	w.WriteHeader(201)
	w.Write(data)
}

func (spy *PKSSpy) Stop() {
	spy.server.Close()
}

func (spy *PKSSpy) Addr() string {
	return spy.server.Addr
}

func (spy *PKSSpy) Port() string {
	_, port, err := net.SplitHostPort(spy.secureConnection.Addr().String())
	if err != nil {
		panic(err)
	}
	return port
}
