package testing

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"sync"

	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func NewIngressClient(addr string) (client rpc.IngressClient, cleanup func()) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(
		GrpcTLSCredentials(),
	))
	if err != nil {
		panic(err)
	}

	return rpc.NewIngressClient(conn), func() {
		conn.Close()
	}
}

func WaitForHealthCheck(healthAddr string) {
	Eventually(func() error {
		resp, err := http.Get("http://" + healthAddr + "/debug/vars")
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("healthcheck returned non-OK status: %d", resp.StatusCode)
		}

		return nil
	}, 5).Should(Succeed())
}

func WaitForServer(addr string) {
	Eventually(func() error {
		resp, err := http.Get("http://" + addr + "/api/v1/labels")
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("api/v1/labels returned non-OK status: %d", resp.StatusCode)
		}

		return nil
	}, 5).Should(Succeed())
}

var builtPathsCache = &sync.Map{}

func StartGoProcess(importPath string, env []string, args ...string) *gexec.Session {
	var commandPath string
	var err error
	cacheValue, ok := builtPathsCache.Load(importPath)

	if ok {
		commandPath = cacheValue.(string)
	} else {
		commandPath, err = gexec.Build(importPath, "-race", "-mod=vendor")
		Expect(err).ToNot(HaveOccurred())

		builtPathsCache.Store(importPath, commandPath)
	}

	command := exec.Command(commandPath, args...)
	command.Env = env
	var session *gexec.Session
	Eventually(func() error {
		var err error
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		return err
	}).Should(Succeed())

	return session
}

func GrpcTLSCredentials() credentials.TransportCredentials {
	credentials, err := tls.NewTLSCredentials(
		Cert("metric-store-ca.crt"),
		Cert("metric-store.crt"),
		Cert("metric-store.key"),
		"metric-store",
	)
	Expect(err).ToNot(HaveOccurred())

	return credentials
}

func GetFreePort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		Expect(err).ToNot(HaveOccurred())
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		Expect(err).ToNot(HaveOccurred())
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
