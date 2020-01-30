package storage_test

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metricstore"
	"github.com/cloudfoundry/metric-store-release/src/internal/storage"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Remote Querier", func() {
	Describe("retry", func() {
		listenForSecureQueries := func(insecureConnection net.Listener) chan int {
			tlsConfig, err := sharedtls.NewMutualTLSServerConfig(
				testing.Cert("metric-store-ca.crt"),
				testing.Cert("metric-store.crt"),
				testing.Cert("metric-store.key"),
			)
			Expect(err).ToNot(HaveOccurred())

			secureConnection := tls.NewListener(insecureConnection, tlsConfig)
			mux := http.NewServeMux()

			calls := make(chan int, 1)
			mux.HandleFunc("/api/v1/read", func(rw http.ResponseWriter, r *http.Request) {
				calls <- 1
				resp := &prompb.ReadResponse{Results: []*prompb.QueryResult{{
					Timeseries: []*prompb.TimeSeries{},
				}}}
				Expect(remote.EncodeReadResponse(resp, rw)).To(Succeed())
			})
			go http.Serve(secureConnection, mux)
			return calls
		}

		defaultQuerierConfig := &config_util.TLSConfig{
			CAFile:     testing.Cert("metric-store-ca.crt"),
			CertFile:   testing.Cert("metric-store.crt"),
			KeyFile:    testing.Cert("metric-store.key"),
			ServerName: metricstore.COMMON_NAME,
		}

		It("connects", func() {
			insecureConnection, err := net.Listen("tcp", ":0")
			defer insecureConnection.Close()
			Expect(err).ToNot(HaveOccurred())

			calls := listenForSecureQueries(insecureConnection)

			ctx, _ := context.WithCancel(context.Background())

			querier, err := storage.NewRemoteQuerier(ctx, 0, insecureConnection.Addr().String(), defaultQuerierConfig, logger.NewTestLogger(GinkgoWriter))
			Expect(err).ToNot(HaveOccurred())
			_, _, err = querier.Select(nil, &labels.Matcher{
				Name:  "__name__",
				Type:  labels.MatchEqual,
				Value: "irrelevantapp",
			})
			Expect(calls).To(Receive())
			Expect(err).ToNot(HaveOccurred())
		})

		It("retries on connection error", func() {
			insecureConnection, err := net.Listen("tcp", ":0")
			Expect(err).ToNot(HaveOccurred())
			insecureConnection.Close()

			internalAddr := insecureConnection.Addr().String()
			go func() {
				querier, err := storage.NewRemoteQuerier(context.Background(), 0, internalAddr, defaultQuerierConfig, logger.NewTestLogger(GinkgoWriter))
				Expect(err).ToNot(HaveOccurred())
				querier.Select(nil, &labels.Matcher{
					Name:  "__name__",
					Type:  labels.MatchEqual,
					Value: "irrelevantapp",
				})
			}()

			time.Sleep(10 * time.Millisecond)

			insecureConnection, err = net.Listen("tcp", internalAddr)
			defer insecureConnection.Close()
			Expect(err).ToNot(HaveOccurred())

			calls := listenForSecureQueries(insecureConnection)

			Eventually(calls).Should(Receive())
		})

		It("respects context", func() {
			insecureConnection, err := net.Listen("tcp", ":0")
			defer insecureConnection.Close()
			Expect(err).ToNot(HaveOccurred())

			calls := listenForSecureQueries(insecureConnection)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			querier, err := storage.NewRemoteQuerier(ctx, 0, insecureConnection.Addr().String(), defaultQuerierConfig, logger.NewTestLogger(GinkgoWriter))
			Expect(err).ToNot(HaveOccurred())
			querier.Select(nil, &labels.Matcher{
				Name:  "__name__",
				Type:  labels.MatchEqual,
				Value: "irrelevantapp",
			})

			Consistently(calls).ShouldNot(Receive())
		})
	})
})
