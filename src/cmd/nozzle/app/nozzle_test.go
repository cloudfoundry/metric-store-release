package app_test

import (
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/cloudfoundry/metric-store-release/src/cmd/nozzle/app"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Nozzle App", func() {
	var (
		loggregator *stubLoggregator
		nozzle      *app.NozzleApp
	)

	BeforeEach(func() {
		loggregator = newStubLoggregator()

		nozzle = app.NewNozzleApp(&app.Config{
			LogProviderAddr: loggregator.addr(),
			LogsProviderTLS: app.LogsProviderTLS{
				LogProviderCA:   testing.Cert("metric-store-ca.crt"),
				LogProviderCert: testing.Cert("localhost.crt"),
				LogProviderKey:  testing.Cert("localhost.key"),
			},
			MetricStoreTLS: app.MetricStoreClientTLS{
				CAPath:   testing.Cert("metric-store-ca.crt"),
				CertPath: testing.Cert("metric-store.crt"),
				KeyPath:  testing.Cert("metric-store.key"),
			},
			MetricStoreMetricsTLS: app.MetricStoreMetricsTLS{
				CAPath:   testing.Cert("metric-store-ca.crt"),
				CertPath: testing.Cert("metric-store.crt"),
				KeyPath:  testing.Cert("metric-store.key"),
			},
		}, logger.NewNop())
		go nozzle.Run()

		Eventually(nozzle.MetricsAddr).ShouldNot(BeEmpty())
	})

	AfterEach(func() {
		defer nozzle.Stop()
	})

	It("serves metrics on a metrics endpoint", func() {
		var body string

		tlsConfig, err := tls.NewMutualTLSClientConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).ToNot(HaveOccurred())

		httpClient := &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		}

		fn := func() string {
			resp, err := httpClient.Get("https://" + nozzle.MetricsAddr() + "/metrics")
			if err != nil {
				return ""
			}
			defer func() { _ = resp.Body.Close() }()

			bytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return ""
			}

			body = string(bytes)

			return body
		}
		Eventually(fn).ShouldNot(BeEmpty())
		Expect(body).To(ContainSubstring(metrics.NozzleIngressEnvelopesTotal))
		Expect(body).To(ContainSubstring("go_threads"))
	})
})

type stubLoggregator struct {
	lis        net.Listener
	grpcServer *grpc.Server
	logStream  chan *loggregator_v2.Envelope
}

func newStubLoggregator() *stubLoggregator {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	tlsConfig, err := loggregator.NewEgressTLSConfig(
		testing.Cert("metric-store-ca.crt"),
		testing.Cert("localhost.crt"),
		testing.Cert("localhost.key"),
	)
	if err != nil {
		panic(err)
	}

	sl := &stubLoggregator{
		lis:        lis,
		grpcServer: grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig))),
		logStream:  make(chan *loggregator_v2.Envelope, 100),
	}

	loggregator_v2.RegisterEgressServer(sl.grpcServer, sl)

	go sl.grpcServer.Serve(lis)

	return sl
}

func (sl *stubLoggregator) Receiver(
	_ *loggregator_v2.EgressRequest,
	_ loggregator_v2.Egress_ReceiverServer,
) error {
	panic("not implemented")
}

func (sl *stubLoggregator) BatchedReceiver(
	_ *loggregator_v2.EgressBatchRequest,
	s loggregator_v2.Egress_BatchedReceiverServer,
) error {
	for env := range sl.logStream {
		_ = s.Send(&loggregator_v2.EnvelopeBatch{
			Batch: []*loggregator_v2.Envelope{env},
		})

		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

func (sl *stubLoggregator) addr() string {
	return sl.lis.Addr().String()
}

func (sl *stubLoggregator) stop() {
	sl.grpcServer.Stop()
}
