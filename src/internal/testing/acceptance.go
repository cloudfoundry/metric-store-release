package testing

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func WaitForHealthCheck(healthPort string, tlsConfig *tls.Config) {
	Eventually(func() error {
		client := &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		}

		resp, err := client.Get("https://localhost:" + healthPort + "/debug/vars")
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("healthcheck returned non-OK status: %d", resp.StatusCode)
		}

		return nil
	}, 5).Should(Succeed())
}

var builtPathsCache = &sync.Map{}

func SkipTestOnMac(){
	if runtime.GOOS == "darwin" {
		Skip("doesn't work on Mac OS")
	}
}

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
