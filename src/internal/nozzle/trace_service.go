package nozzle

import (
	"code.cloudfoundry.org/go-diodes"
	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/nozzle/rollup"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"strconv"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"

	_ "google.golang.org/grpc/encoding/gzip"

	"golang.org/x/net/context"

	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	ot "go.opentelemetry.io/proto/otlp/trace/v1"
)

type TraceService struct {
	tracepb.TraceServiceServer

	log           *logger.Logger
	client        *ingressclient.IngressClient
	metrics       metrics.Registrar
	allowListTags []string
	filterTags    bool
	pointBuffer   chan []*rpc.Point

	nodeIndex             int
	timerRollupBufferSize uint
	rollupInterval        time.Duration
	totalRollup           rollup.Rollup
	durationRollup        rollup.Rollup

	timerBuffer *diodes.OneToOne

	ctx    context.Context
	cancel func()
	done   chan struct{}
}

type TraceServiceOptions func(*TraceService)

func NewTraceService(client *ingressclient.IngressClient, nodeIndex int, opts ...TraceServiceOptions) *TraceService {
	ctx, cancel := context.WithCancel(context.Background())

	t := &TraceService{
		log:                   logger.NewNop(),
		metrics:               &metrics.NullRegistrar{},
		client:                client,
		nodeIndex:             nodeIndex,
		timerRollupBufferSize: 4096,
		totalRollup:           rollup.NewNullRollup(),
		durationRollup:        rollup.NewNullRollup(),
		pointBuffer:           make(chan []*rpc.Point, BATCH_CHANNEL_SIZE),

		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}, 1)}

	for _, o := range opts {
		o(t)
	}

	t.timerBuffer = diodes.NewOneToOne(int(t.timerRollupBufferSize), diodes.AlertFunc(func(missed int) {
		t.metrics.Add(metrics.OtelDroppedSpansTotal, float64(missed))
		t.log.Info("timer buffer dropped spans", logger.Count(missed))
	}))

	return t
}

func WithTraceFiltering(allowListTags []string, filterToAppMetrics bool) TraceServiceOptions {
	return func(s *TraceService) {
		s.allowListTags = allowListTags
		s.filterTags = filterToAppMetrics
	}
}

func WithTraceOtelLogger(l *logger.Logger) TraceServiceOptions {
	return func(s *TraceService) {
		s.log = l
	}
}
func WithTraceOtelDebugRegistrar(m metrics.Registrar) TraceServiceOptions {
	return func(s *TraceService) {
		s.metrics = m
	}
}

func WithOtelTimerRollupBufferSize(size uint) TraceServiceOptions {
	return func(s *TraceService) {
		s.timerRollupBufferSize = size
	}
}

func WithOtelTimerRollup(interval time.Duration, totalRollupTags, durationRollupTags []string) TraceServiceOptions {
	return func(s *TraceService) {
		s.rollupInterval = interval

		nodeIndex := strconv.Itoa(s.nodeIndex)
		s.totalRollup = rollup.NewCounterRollup(s.log, nodeIndex, totalRollupTags)
		s.durationRollup = rollup.NewHistogramRollup(s.log, nodeIndex, durationRollupTags)
	}
}

func (s *TraceService) StartListening() {
	defer func() {
		close(s.done)
	}()

	s.log.Info("starting trace service")

	go s.timerProcessor()
	go s.timerRollup()
	go s.saveToStore()
}

func (s *TraceService) Stop() {
	s.cancel()
	<-s.done
}

// Function called with registry of TraceServiceServer
func (s *TraceService) Export(_ context.Context, req *tracepb.ExportTraceServiceRequest) (*tracepb.ExportTraceServiceResponse, error) {
	for _, rm := range req.ResourceSpans {
		for _, ilm := range rm.ScopeSpans {
			for _, sp := range ilm.Spans {
				s.timerBuffer.Set(diodes.GenericDataType(sp))
				s.metrics.Inc(metrics.OtelIngressSpansTotal)
			}
		}
	}
	return &tracepb.ExportTraceServiceResponse{}, nil
}

func (s *TraceService) convertToSpan(sp *ot.Span) *rpc.Span {
	tags := map[string]string{}

	for _, tag := range sp.GetAttributes() {
		tags[tag.Key] = tag.Value.GetStringValue()
	}

	if !s.allowListedTrace(tags) {
		sourceId := tags["source_id"]
		s.log.Log("msg", "TraceService.denied span:", "source_id", sourceId)

		s.metrics.Inc(metrics.OtelDeniedSpansTotal)
		return nil
	}

	return &rpc.Span{
		Timestamp: int64(sp.StartTimeUnixNano),
		SourceId:  tags["source_id"],
		Duration:  int64(sp.EndTimeUnixNano - sp.StartTimeUnixNano),
		Labels:    tags,
	}
}

func (s *TraceService) addToRollup(span *rpc.Span) {
	s.log.Log("msg", "TraceService.addToRollup:", "span", span)

	s.totalRollup.Record(span.SourceId, span.Labels, 1)
	s.durationRollup.Record(span.SourceId, span.Labels, span.Duration)
}

func (s *TraceService) timerProcessor() {
	poller := diodes.NewPoller(s.timerBuffer)

	for {
		data := poller.Next()
		rawSpan := (*ot.Span)(data)
		span := s.convertToSpan(rawSpan)
		if span != nil {
			s.addToRollup(span)
		}
	}
}

func (s *TraceService) timerRollup() {
	onInterval := time.NewTicker(s.rollupInterval)
	defer onInterval.Stop()
	for {
		select {
		case t := <-onInterval.C:
			timestampNano := t.Truncate(s.rollupInterval).UnixNano()

			var points []*rpc.Point

			for _, pointsBatch := range s.totalRollup.Rollup(timestampNano) {
				points = append(points, pointsBatch.Points...)
			}

			for _, pointsBatch := range s.durationRollup.Rollup(timestampNano) {
				points = append(points, pointsBatch.Points...)
			}

			if len(points) > 0 {
				points = s.writeToChannelOrDiscard(points)
			}
		}
	}
}

func (s *TraceService) writeToChannelOrDiscard(points []*rpc.Point) []*rpc.Point {
	select {
	case s.pointBuffer <- points:
		return make([]*rpc.Point, 0)
	default:
		// if we can't write into the channel, it must be full, so
		// we probably need to drop these envelopes on the floor
		s.log.Info("TraceService: cannot write points. dropping points")
		s.metrics.Add(metrics.OtelDroppedSpansTotal, float64(len(points)))
		return points[:0]
	}
}

func (s *TraceService) saveToStore() {
	for {
		points := <-s.pointBuffer
		s.log.Info("TraceService: writing points to metrics store")
		start := time.Now()
		err := s.client.Write(points)
		if err != nil {
			s.log.Error("TraceService: Error writing to metric-store otel", err)
			s.metrics.Inc(metrics.OtelEgressSpansErrorsTotal)
			continue
		}
		s.metrics.Histogram(metrics.OtelEgressSpansDurationSeconds).Observe(transform.DurationToSeconds(time.Since(start)))
		s.metrics.Add(metrics.OtelEgressSpansTotal, float64(len(points)))
	}
}
func (s *TraceService) allowListedTrace(tags map[string]string) bool {

	for _, t := range s.allowListTags {
		if _, hasTagKey := tags[t]; hasTagKey {
			return true
		}
	}
	return false
}
