package nozzle

import (
	"crypto/tls"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/nozzle/rollup"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"

	"code.cloudfoundry.org/go-diodes"
	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"golang.org/x/net/context"
)

// Nozzle reads envelopes and writes points to metric-store.
type Nozzle struct {
	log     *logger.Logger
	metrics metrics.Registrar

	s                      StreamConnector
	shardId                string
	nodeIndex              int
	enableEnvelopeSelector bool
	envelopSelectorTags    []string
	ingressBuffer          *diodes.OneToOne

	timerBuffer           *diodes.OneToOne
	timerRollupBufferSize uint
	rollupInterval        time.Duration
	totalRollup           rollup.Rollup
	durationRollup        rollup.Rollup

	ingressAddr string
	client      *ingressclient.IngressClient
	tlsConfig   *tls.Config
	pointBuffer chan []*rpc.Point
}

// StreamConnector reads envelopes from the the logs provider.
type StreamConnector interface {
	// Stream creates a EnvelopeStream for the given request.
	Stream(ctx context.Context, req *loggregator_v2.EgressBatchRequest) loggregator.EnvelopeStream
}

const (
	BATCH_FLUSH_INTERVAL = 500 * time.Millisecond
	BATCH_CHANNEL_SIZE   = 512
)

func NewNozzle(c StreamConnector, ingressAddr string, tlsConfig *tls.Config, shardId string, nodeIndex int, enableEnvelopeSelector bool, envelopSelectorTags []string, opts ...Option) *Nozzle {
	n := &Nozzle{
		log:                    logger.NewNop(),
		metrics:                &metrics.NullRegistrar{},
		s:                      c,
		shardId:                shardId,
		nodeIndex:              nodeIndex,
		enableEnvelopeSelector: enableEnvelopeSelector,
		envelopSelectorTags:    envelopSelectorTags,
		timerRollupBufferSize:  4096,
		totalRollup:            rollup.NewNullRollup(),
		durationRollup:         rollup.NewNullRollup(),
		ingressAddr:            ingressAddr,
		tlsConfig:              tlsConfig,
		pointBuffer:            make(chan []*rpc.Point, BATCH_CHANNEL_SIZE),
	}

	for _, o := range opts {
		o(n)
	}

	client, err := ingressclient.NewIngressClient(
		ingressAddr,
		tlsConfig,
		ingressclient.WithIngressClientLogger(n.log),
		ingressclient.WithDialTimeout(time.Minute),
	)
	if err != nil {
		n.log.Panic(
			"Could not connect to ingress server",
			logger.String("address", ingressAddr),
		)
	}
	n.client = client

	n.timerBuffer = diodes.NewOneToOne(int(n.timerRollupBufferSize), diodes.AlertFunc(func(missed int) {
		n.metrics.Add(metrics.NozzleDroppedEnvelopesTotal, float64(missed))
		n.log.Info("timer buffer dropped points", logger.Count(missed))
	}))

	n.ingressBuffer = diodes.NewOneToOne(100000, diodes.AlertFunc(func(missed int) {
		n.metrics.Add(metrics.NozzleDroppedEnvelopesTotal, float64(missed))
		n.log.Info("ingress buffer dropped envelopes", logger.Count(missed))
	}))

	return n
}

type Option func(*Nozzle)

// WithNozzleLogger returns a Option that configures a nozzle's logger.
// It defaults to silent logging.
func WithNozzleLogger(l *logger.Logger) Option {
	return func(n *Nozzle) {
		n.log = l
	}
}

func WithNozzleDebugRegistrar(m metrics.Registrar) Option {
	return func(n *Nozzle) {
		n.metrics = m
	}
}

func WithNozzleTimerRollupBufferSize(size uint) Option {
	return func(n *Nozzle) {
		n.timerRollupBufferSize = size
	}
}

func WithNozzleTimerRollup(interval time.Duration, totalRollupTags, durationRollupTags []string) Option {
	return func(n *Nozzle) {
		n.rollupInterval = interval

		nodeIndex := strconv.Itoa(n.nodeIndex)
		n.totalRollup = rollup.NewCounterRollup(n.log, nodeIndex, totalRollupTags)
		// TODO: rename HistogramRollup
		n.durationRollup = rollup.NewHistogramRollup(n.log, nodeIndex, durationRollupTags)
	}
}

// Start() starts reading envelopes from the logs provider and writes them to
// metric-store.
func (n *Nozzle) Start() {
	rx := n.s.Stream(context.Background(), n.buildBatchReq())

	go n.timerProcessor()
	go n.timerEmitter()
	go n.envelopeReader(rx)

	n.log.Info("starting workers", logger.Count(2*runtime.NumCPU()))
	for i := 0; i < 2*runtime.NumCPU(); i++ {
		go n.pointWriter()
	}

	go n.pointBatcher()
}

func (n *Nozzle) pointBatcher() {
	var size int

	poller := diodes.NewPoller(n.ingressBuffer)
	points := make([]*rpc.Point, 0)

	t := time.NewTimer(BATCH_FLUSH_INTERVAL)
	for {
		data, found := poller.TryNext()

		if found {
			for _, point := range n.convertEnvelopeToPoints((*loggregator_v2.Envelope)(data)) {
				size += point.EstimatePointSize()
				points = append(points, point)
			}
		}

		select {
		case <-t.C:
			if len(points) > 0 {
				points = n.writeToChannelOrDiscard(points)
			}
			t.Reset(BATCH_FLUSH_INTERVAL)
			size = 0
		default:
			// Do we care if one envelope produces multiple points, in which a
			// subset crosses the threshold?

			// if len(points) >= BATCH_CHANNEL_SIZE {
			if size >= ingressclient.MAX_BATCH_SIZE_IN_BYTES {
				points = n.writeToChannelOrDiscard(points)
				t.Reset(BATCH_FLUSH_INTERVAL)
				size = 0
			}

			// this sleep keeps us from hammering an empty channel, which
			// would otherwise cause us to peg the cpu when there's no work
			// to be done.
			if !found {
				time.Sleep(time.Millisecond)
			}
		}
	}
}

func (n *Nozzle) writeToChannelOrDiscard(points []*rpc.Point) []*rpc.Point {
	select {
	case n.pointBuffer <- points:
		return make([]*rpc.Point, 0)
	default:
		// if we can't write into the channel, it must be full, so
		// we probably need to drop these envelopes on the floor
		n.metrics.Add(metrics.NozzleDroppedPointsTotal, float64(len(points)))
		return points[:0]
	}
}

func (n *Nozzle) pointWriter() {
	for {
		points := <-n.pointBuffer
		start := time.Now()

		err := n.client.Write(points)
		if err != nil {
			n.log.Error("Error writing to metric-store", err)
			n.metrics.Inc(metrics.NozzleEgressErrorsTotal)
			continue
		}

		n.metrics.Histogram(metrics.NozzleEgressDurationSeconds).Observe(transform.DurationToSeconds(time.Since(start)))
		n.metrics.Add(metrics.NozzleEgressPointsTotal, float64(len(points)))
	}
}

func (n *Nozzle) envelopeReader(rx loggregator.EnvelopeStream) {
	for {
		envelopeBatch := rx()
		for _, envelope := range envelopeBatch {
			n.ingressBuffer.Set(diodes.GenericDataType(envelope))
			n.metrics.Inc(metrics.NozzleIngressEnvelopesTotal)
		}
	}
}

func (n *Nozzle) timerProcessor() {
	poller := diodes.NewPoller(n.timerBuffer)

	for {
		data := poller.Next()
		envelope := *(*loggregator_v2.Envelope)(data)
		timer := envelope.GetTimer()

		n.totalRollup.Record(envelope.SourceId, envelope.Tags, 1)
		n.durationRollup.Record(envelope.SourceId, envelope.Tags, timer.GetStop()-timer.GetStart())
	}
}

func (n *Nozzle) timerEmitter() {
	ticker := time.NewTicker(n.rollupInterval)

	for t := range ticker.C {
		timestampNano := t.Truncate(n.rollupInterval).UnixNano()

		var size int
		var points []*rpc.Point

		for _, pointsBatch := range n.totalRollup.Rollup(timestampNano) {
			points = append(points, pointsBatch.Points...)
			size += pointsBatch.Size

			if size >= ingressclient.MAX_BATCH_SIZE_IN_BYTES {
				points = n.writeToChannelOrDiscard(points)
				size = 0
			}
		}

		for _, pointsBatch := range n.durationRollup.Rollup(timestampNano) {
			points = append(points, pointsBatch.Points...)
			size += pointsBatch.Size

			if size >= ingressclient.MAX_BATCH_SIZE_IN_BYTES {
				points = n.writeToChannelOrDiscard(points)
				size = 0
			}
		}

		if len(points) > 0 {
			points = n.writeToChannelOrDiscard(points)
		}
	}
}

func (n *Nozzle) convertEnvelopesToPoints(envelopes []*loggregator_v2.Envelope) []*rpc.Point {
	var points []*rpc.Point

	for _, envelope := range envelopes {
		points = append(points, n.convertEnvelopeToPoints(envelope)...)
	}
	return points
}

func (n *Nozzle) captureGorouterHttpTimerMetricsForRollup(envelope *loggregator_v2.Envelope) {
	timer := envelope.GetTimer()

	if timer.GetName() != rollup.GorouterHttpMetricName {
		return
	}

	if envelope.GetSourceId() == rollup.GorouterSourceId {
		if strings.ToLower(envelope.Tags["peer_type"]) == "client" {
			// gorouter reports both client and server timers for each request,
			// only record server types
			return
		}
	} else {
		if !n.hasMatchedTags(envelope.GetTags()) {
			return
		}
	}

	n.timerBuffer.Set(diodes.GenericDataType(envelope))
}

func (n *Nozzle) convertEnvelopeToPoints(envelope *loggregator_v2.Envelope) []*rpc.Point {
	switch envelope.Message.(type) {
	case *loggregator_v2.Envelope_Gauge:
		return n.createPointsFromGauge(envelope)
	case *loggregator_v2.Envelope_Timer:
		n.captureGorouterHttpTimerMetricsForRollup(envelope)
		return []*rpc.Point{}
	case *loggregator_v2.Envelope_Counter:
		if point := n.createPointFromCounter(envelope); point != nil {
			return []*rpc.Point{point}
		}
		return []*rpc.Point{}
	}

	return []*rpc.Point{}
}

// checks if enabled configuration enableEnvelopeSelector keep only application metrics and specified tags metrics
// return true when need to drop this envelop
func (n *Nozzle) hasMatchedTags(tags map[string]string) bool {
	if n.enableEnvelopeSelector {
		for _, t := range n.envelopSelectorTags {
			if _, hasTagKey := tags[t]; hasTagKey {
				return true
			}
		}

		n.metrics.Inc(metrics.NozzleSkippedEnvelopsByTagTotal)
		return false
	}

	return true
}

func (n *Nozzle) createPointsFromGauge(envelope *loggregator_v2.Envelope) []*rpc.Point {
	var points []*rpc.Point
	gauge := envelope.GetGauge()
	for name, metric := range gauge.GetMetrics() {
		labels := map[string]string{
			"source_id": envelope.GetSourceId(),
			"unit":      metric.GetUnit(),
		}

		tags := envelope.GetTags()
		if !n.hasMatchedTags(tags) {
			return nil
		}

		for k, v := range envelope.GetTags() {
			labels[k] = v
		}
		point := &rpc.Point{
			Timestamp: envelope.GetTimestamp(),
			Name:      name,
			Value:     metric.GetValue(),
			Labels:    labels,
		}
		points = append(points, point)
	}

	return points
}

func (n *Nozzle) createPointFromCounter(envelope *loggregator_v2.Envelope) *rpc.Point {
	counter := envelope.GetCounter()
	labels := map[string]string{
		"source_id": envelope.GetSourceId(),
	}

	tags := envelope.GetTags()
	if !n.hasMatchedTags(tags) {
		return nil
	}

	for k, v := range envelope.GetTags() {
		labels[k] = v
	}
	return &rpc.Point{
		Timestamp: envelope.GetTimestamp(),
		Name:      counter.GetName(),
		Value:     float64(counter.GetTotal()),
		Labels:    labels,
	}
}

var selectorTypes = []*loggregator_v2.Selector{
	{
		Message: &loggregator_v2.Selector_Gauge{
			Gauge: &loggregator_v2.GaugeSelector{},
		},
	},
	{
		Message: &loggregator_v2.Selector_Counter{
			Counter: &loggregator_v2.CounterSelector{},
		},
	},
	{
		Message: &loggregator_v2.Selector_Timer{
			Timer: &loggregator_v2.TimerSelector{},
		},
	},
}

func (n *Nozzle) buildBatchReq() *loggregator_v2.EgressBatchRequest {
	return &loggregator_v2.EgressBatchRequest{
		ShardId:          n.shardId,
		UsePreferredTags: true,
		Selectors:        selectorTypes,
	}
}
