package local_test

import (
	"context"
	"io/ioutil"
	"log"
	"sync"

	"github.com/cloudfoundry/metric-store/src/pkg/local"
	rpc "github.com/cloudfoundry/metric-store/src/pkg/rpc/metricstore_v1"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IngressReverseProxy", func() {
	var (
		p                     *local.IngressReverseProxy
		spyIngressLocalClient *spyIngressClient
	)

	BeforeEach(func() {
		spyIngressLocalClient = newSpyIngressClient()
		p = local.NewIngressReverseProxy(
			spyIngressLocalClient,
			log.New(ioutil.Discard, "", 0),
		)
	})

	It("sanitizes metric and label names", func() {
		_, err := p.Send(context.Background(), &rpc.SendRequest{
			Batch: &rpc.Points{
				Points: []*rpc.Point{
					{Name: "some_name_a", Timestamp: 1},
					{Name: "some-name-b", Timestamp: 2, Labels: map[string]string{"f:oo": "bar"}},
					{Name: "some.name.c", Timestamp: 3, Labels: map[string]string{"f.oo": "bar"}},
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(spyIngressLocalClient.reqs).To(ConsistOf(
			&rpc.SendRequest{
				Batch: &rpc.Points{
					Points: []*rpc.Point{
						{Name: "some_name_a", Timestamp: 1},
						{Name: "some_name_b", Timestamp: 2, Labels: map[string]string{"f_oo": "bar"}},
						{Name: "some_name_c", Timestamp: 3, Labels: map[string]string{"f_oo": "bar"}},
					},
				},
			},
		))
	})

	It("uses the given context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := p.Send(ctx, &rpc.SendRequest{
			Batch: &rpc.Points{
				Points: []*rpc.Point{
					{Name: "some-name", Timestamp: 1},
					{Name: "some-name", Timestamp: 2},
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(spyIngressLocalClient.ctxs).ToNot(BeEmpty())
		Expect(spyIngressLocalClient.ctxs[0].Done()).To(BeClosed())
	})
})

type spyIngressClient struct {
	mu   sync.Mutex
	ctxs []context.Context
	reqs []*rpc.SendRequest
	err  error
}

func newSpyIngressClient() *spyIngressClient {
	return &spyIngressClient{}
}

func (s *spyIngressClient) Send(ctx context.Context, in *rpc.SendRequest, opts ...grpc.CallOption) (*rpc.SendResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ctxs = append(s.ctxs, ctx)
	s.reqs = append(s.reqs, in)
	return &rpc.SendResponse{}, s.err
}

func (s *spyIngressClient) Requests() []*rpc.SendRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := make([]*rpc.SendRequest, len(s.reqs))
	copy(r, s.reqs)
	return r
}
