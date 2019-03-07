package local

import (
	"context"
	"log"

	"github.com/cloudfoundry/metric-store/pkg/persistence/transform"
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
	var points []*rpc.Point

	for _, point := range r.Batch.Points {
		point.Name = transform.SanitizeMetricName(point.GetName())

		sanitizedLabels := make(map[string]string)
		for label, value := range point.GetLabels() {
			sanitizedLabels[transform.SanitizeLabelName(label)] = value
		}
		if len(sanitizedLabels) > 0 {
			point.Labels = sanitizedLabels
		}

		points = append(points, point)
	}

	return p.localClient.Send(ctx, &rpc.SendRequest{
		Batch: &rpc.Points{
			Points: points,
		},
	})
}

// IngressClientFunc transforms a function into an IngressClient.
type IngressClientFunc func(ctx context.Context, r *rpc.SendRequest, opts ...grpc.CallOption) (*rpc.SendResponse, error)

// Send implements an IngressClient.
func (f IngressClientFunc) Send(ctx context.Context, r *rpc.SendRequest, opts ...grpc.CallOption) (*rpc.SendResponse, error) {
	return f(ctx, r, opts...)
}
