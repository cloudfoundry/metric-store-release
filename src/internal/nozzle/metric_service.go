package nozzle

import (
	"code.cloudfoundry.org/go-diodes"
	"errors"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"runtime"
	"time"

	_ "google.golang.org/grpc/encoding/gzip"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	metricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	otm "go.opentelemetry.io/proto/otlp/metrics/v1"
	"golang.org/x/net/context"
)

type MetricService struct {
	metricspb.MetricsServiceServer

	log           *logger.Logger
	client        *ingressclient.IngressClient
	allowListTags []string
	filterMetrics bool
	ingressBuffer *diodes.OneToOne
	pointBuffer   chan []*rpc.Point
	metrics       metrics.Registrar

	ctx    context.Context
	cancel func()
	done   chan struct{}
}

type MetricServiceOptions func(*MetricService)

func WithMetricOtelLogger(l *logger.Logger) MetricServiceOptions {
	return func(s *MetricService) {
		s.log = l
	}
}
func WithMetricOtelDebugRegistrar(m metrics.Registrar) MetricServiceOptions {
	return func(s *MetricService) {
		s.metrics = m
	}
}

func WithMetricFiltering(allowListTags []string, filterToAppMetrics bool) MetricServiceOptions {
	return func(s *MetricService) {
		s.allowListTags = allowListTags
		s.filterMetrics = filterToAppMetrics
	}
}

func NewMetricService(client *ingressclient.IngressClient, opts ...MetricServiceOptions) *MetricService {
	ctx, cancel := context.WithCancel(context.Background())

	m := &MetricService{
		log:         logger.NewNop(),
		client:      client,
		metrics:     &metrics.NullRegistrar{},
		pointBuffer: make(chan []*rpc.Point, BATCH_CHANNEL_SIZE),

		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}, 1),
	}

	for _, o := range opts {
		o(m)
	}

	m.ingressBuffer = diodes.NewOneToOne(100000, diodes.AlertFunc(func(missed int) {
		m.metrics.Add(metrics.OtelDroppedMetricsTotal, float64(missed))
		m.log.Info("ingress buffer dropped metrics", logger.Count(missed))
	}))

	return m
}

func (s *MetricService) StartListening() {
	defer func() {
		close(s.done)
	}()

	s.log.Info("Starting otel metric workers", logger.Count(runtime.NumCPU()))
	for i := 0; i < runtime.NumCPU(); i++ {
		go s.pointWriter()
	}
	go s.metricProcessor()
}

func (s *MetricService) Stop() {
	s.cancel()
	<-s.done
}

// Export processes received metric data.
func (s *MetricService) Export(_ context.Context, req *metricspb.ExportMetricsServiceRequest) (*metricspb.ExportMetricsServiceResponse, error) {
	// Iterate over the resource metrics and their scope metrics
	for _, rm := range req.ResourceMetrics {
		for _, ilm := range rm.ScopeMetrics {
			for _, metric := range ilm.Metrics {
				s.ingressBuffer.Set(diodes.GenericDataType(metric))
			}
		}
	}
	return &metricspb.ExportMetricsServiceResponse{}, nil
}

func (s *MetricService) metricProcessor() {
	poller := diodes.NewPoller(s.ingressBuffer)

	for {
		data := poller.Next()
		metric := (*otm.Metric)(data)
		points := s.convertMetricToPoints(metric)
		if len(points) > 0 {
			s.writeToChannelOrDiscard(points)
		}
	}
}

func (s *MetricService) writeToChannelOrDiscard(points []*rpc.Point) []*rpc.Point {
	select {
	case s.pointBuffer <- points:
		return make([]*rpc.Point, 0)
	default:
		s.log.Info("cannot write points. dropping points")
		s.metrics.Add(metrics.OtelDroppedMetricsTotal, float64(len(points)))

		return points[:0]
	}
}

func (s *MetricService) pointWriter() {
	for {
		points := <-s.pointBuffer
		start := time.Now()
		err := s.client.Write(points)
		if err != nil {
			s.log.Error("Error writing to metric-store otel", err)
			s.metrics.Inc(metrics.OtelEgressMetricsErrorsTotal)
			continue
		}
		s.metrics.Histogram(metrics.OtelEgressMetricsDurationSeconds).Observe(transform.DurationToSeconds(time.Since(start)))
		s.metrics.Add(metrics.OtelEgressMetricsTotal, float64(len(points)))
	}
}

func (s *MetricService) convertMetricToPoints(metric *otm.Metric) []*rpc.Point {
	switch metric.Data.(type) {
	case *otm.Metric_Gauge:
		return s.createPointsFromGauge(metric)
	case *otm.Metric_Sum:
		return s.createPointsFromCounter(metric)
	case *otm.Metric_Histogram:
		s.log.Info("NotImplemented -- Metric_Histogram")
		return []*rpc.Point{}
	case *otm.Metric_ExponentialHistogram:
		s.log.Info("NotImplemented -- Metric_ExponentialHistogram")
		return []*rpc.Point{}
	case *otm.Metric_Summary:
		s.log.Info("NotImplemented -- Metric_Summary")
		return []*rpc.Point{}
	}

	return []*rpc.Point{}
}

func (s *MetricService) createPointsFromMetric(metric *otm.Metric, dataPoints []*otm.NumberDataPoint) []*rpc.Point {

	var points []*rpc.Point
	for _, rp := range dataPoints {
		labels := map[string]string{
			"unit": metric.GetUnit(),
		}

		attributes := rp.GetAttributes()
		for _, tag := range attributes {
			labels[tag.Key] = tag.Value.GetStringValue()
		}

		if !s.AllowListedMetric(labels) {
			s.metrics.Inc(metrics.OtelDeniedMetricsTotal)
			return []*rpc.Point{}
		}

		val, err := s.getPointValue(rp)
		if err != nil {
			s.log.Info("bad type for gauge data rp")
			continue
		}
		point := &rpc.Point{
			Timestamp: int64(rp.GetTimeUnixNano()),
			Name:      metric.Name,
			Value:     val,
			Labels:    labels,
		}
		s.metrics.Inc(metrics.OtelIngressMetricsTotal)

		s.log.Log("Point:", point)
		points = append(points, point)
	}

	return points
}

func (s *MetricService) createPointsFromGauge(metric *otm.Metric) []*rpc.Point {

	gauge := metric.GetGauge()
	if gauge == nil {
		return []*rpc.Point{}
	}

	return s.createPointsFromMetric(metric, gauge.GetDataPoints())
}

func (s *MetricService) createPointsFromCounter(metric *otm.Metric) []*rpc.Point {

	counter := metric.GetSum()
	if counter == nil {
		return []*rpc.Point{}
	}

	return s.createPointsFromMetric(metric, counter.GetDataPoints())
}

func (s *MetricService) getPointValue(point *otm.NumberDataPoint) (float64, error) {
	switch v := point.Value.(type) {
	case *otm.NumberDataPoint_AsDouble:
		return v.AsDouble, nil
	case *otm.NumberDataPoint_AsInt:
		return float64(v.AsInt), nil
	}
	return 0, errors.New("bad type for gauge data point")
}

func (s *MetricService) AllowListedMetric(tags map[string]string) bool {
	//This is a feature flag to only store app metrics and no container metrics
	if !s.filterMetrics {
		return true
	}

	for _, t := range s.allowListTags {
		if _, hasTagKey := tags[t]; hasTagKey {
			return true
		}
	}
	return false
}
