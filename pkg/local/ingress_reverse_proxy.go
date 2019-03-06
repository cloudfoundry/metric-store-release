package local

import (
	"context"
	"log"

	rpc "github.com/cloudfoundry/metric-store/pkg/rpc/metricstore_v1"
	"google.golang.org/grpc"
)

// IngressReverseProxy is a reverse proxy for Ingress requests.
type IngressReverseProxy struct {
	localClient rpc.IngressClient
	log         *log.Logger
}

// NewIngressReverseProxy returns a new IngressReverseProxy.
func NewIngressReverseProxy(
	localClient rpc.IngressClient,
	log *log.Logger,
) *IngressReverseProxy {

	return &IngressReverseProxy{
		localClient: localClient,
		log:         log,
	}
}

func (p *IngressReverseProxy) Send(ctx context.Context, r *rpc.SendRequest) (*rpc.SendResponse, error) {
	return p.localClient.Send(ctx, r)
}

// IngressClientFunc transforms a function into an IngressClient.
type IngressClientFunc func(ctx context.Context, r *rpc.SendRequest, opts ...grpc.CallOption) (*rpc.SendResponse, error)

// Send implements an IngressClient.
func (f IngressClientFunc) Send(ctx context.Context, r *rpc.SendRequest, opts ...grpc.CallOption) (*rpc.SendResponse, error) {
	return f(ctx, r, opts...)
}
