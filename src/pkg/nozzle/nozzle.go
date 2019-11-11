package nozzle

import (
	"crypto/tls"
	"runtime"
	"strconv"
	"strings"
	"time"

	diodes "code.cloudfoundry.org/go-diodes"
	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/cloudfoundry/metric-store-release/src/pkg/debug"
	"github.com/cloudfoundry/metric-store-release/src/pkg/ingressclient"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/nozzle/rollup"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

// Nozzle reads envelopes and writes points to metric-store.
type Nozzle struct {
	log     *logger.Logger
	metrics debug.MetricRegistrar

	s             StreamConnector
	shardId       string
	nodeIndex     int
	ingressBuffer *diodes.OneToOne

	timerBuffer           *diodes.OneToOne
	timerRollupBufferSize uint
	rollupInterval        time.Duration
	rollupMetricName      string
	totalRollup           rollup.Rollup
	durationRollup        rollup.Rollup

	addr        string
	ingressAddr string
	client      *ingressclient.IngressClient
	tlsConfig   *tls.Config
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

func NewNozzle(c StreamConnector, metricStoreAddr, ingressAddr string, tlsConfig *tls.Config, shardId string, nodeIndex int, opts ...NozzleOption) *Nozzle {
	n := &Nozzle{
		log:                   logger.NewNop(),
		metrics:               &debug.NullRegistrar{},
		s:                     c,
		shardId:               shardId,
		nodeIndex:             nodeIndex,
		timerRollupBufferSize: 4096,
		totalRollup:           rollup.NewNullRollup(),
		durationRollup:        rollup.NewNullRollup(),
		addr:                  metricStoreAddr,
		ingressAddr:           ingressAddr,
		tlsConfig:             tlsConfig,
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
			zap.String("address", ingressAddr),
		)
	}
	n.client = client

	n.timerBuffer = diodes.NewOneToOne(int(n.timerRollupBufferSize), diodes.AlertFunc(func(missed int) {
		n.metrics.Add(debug.NozzleDroppedEnvelopesTotal, float64(missed))
		n.log.Info("timer buffer dropped points", logger.Count(missed))
	}))

	n.ingressBuffer = diodes.NewOneToOne(100000, diodes.AlertFunc(func(missed int) {
		n.metrics.Add(debug.NozzleDroppedEnvelopesTotal, float64(missed))
		n.log.Info("ingress buffer dropped envelopes", logger.Count(missed))
	}))

	return n
}

type NozzleOption func(*Nozzle)

// WithNozzleLogger returns a NozzleOption that configures a nozzle's logger.
// It defaults to silent logging.
func WithNozzleLogger(l *logger.Logger) NozzleOption {
	return func(n *Nozzle) {
		n.log = l
	}
}

func WithNozzleDebugRegistrar(m debug.MetricRegistrar) NozzleOption {
	return func(n *Nozzle) {
		n.metrics = m
	}
}

func WithNozzleTimerRollupBufferSize(size uint) NozzleOption {
	return func(n *Nozzle) {
		n.timerRollupBufferSize = size
	}
}

func WithNozzleTimerRollup(interval time.Duration, metricName string, totalRollupTags, durationRollupTags []string) NozzleOption {
	return func(n *Nozzle) {
		n.rollupInterval = interval
		n.rollupMetricName = metricName

		nodeIndex := strconv.Itoa(n.nodeIndex)
		n.totalRollup = rollup.NewCounterRollup(n.log, nodeIndex, metricName, totalRollupTags)
		n.durationRollup = rollup.NewHistogramRollup(n.log, nodeIndex, metricName, durationRollupTags)
	}
}

// Start() starts reading envelopes from the logs provider and writes them to
// metric-store. It blocks indefinitely.
func (n *Nozzle) Start() {
	rx := n.s.Stream(context.Background(), n.buildBatchReq())

	ch := make(chan []*rpc.Point, BATCH_CHANNEL_SIZE)

	go n.timerProcessor()
	go n.timerEmitter(ch)
	go n.envelopeReader(rx)

	n.log.Info("starting workers", logger.Count(2*runtime.NumCPU()))
	for i := 0; i < 2*runtime.NumCPU(); i++ {
		go n.pointWriter(ch)
	}

	// The batcher will block indefinitely.
	n.pointBatcher(ch)
}

func (n *Nozzle) pointBatcher(ch chan []*rpc.Point) {
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
				points = n.writeToChannelOrDiscard(ch, points)
			}
			t.Reset(BATCH_FLUSH_INTERVAL)
			size = 0
		default:
			// Do we care if one envelope procuces multiple points, in which a
			// subset crosses the threshold?

			// if len(points) >= BATCH_CHANNEL_SIZE {
			if size >= ingressclient.MAX_BATCH_SIZE_IN_BYTES {
				points = n.writeToChannelOrDiscard(ch, points)
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

func (n *Nozzle) writeToChannelOrDiscard(ch chan []*rpc.Point, points []*rpc.Point) []*rpc.Point {
	select {
	case ch <- points:
		return make([]*rpc.Point, 0)
	default:
		// if we can't write into the channel, it must be full, so
		// we probably need to drop these envelopes on the floor
		n.metrics.Add(debug.NozzleDroppedPointsTotal, float64(len(points)))
		return points[:0]
	}
}

func (n *Nozzle) pointWriter(ch chan []*rpc.Point) {
	for {
		points := <-ch
		start := time.Now()

		err := n.client.Write(points)
		if err != nil {
			n.log.Error("Error writing to metric-store", err)
			n.metrics.Inc(debug.NozzleEgressErrorsTotal)
			continue
		}

		n.metrics.Set(debug.NozzleEgressDurationSeconds, transform.DurationToSeconds(time.Since(start)))
		n.metrics.Add(debug.NozzleEgressPointsTotal, float64(len(points)))
	}
}

func (n *Nozzle) envelopeReader(rx loggregator.EnvelopeStream) {
	for {
		envelopeBatch := rx()
		for _, envelope := range envelopeBatch {
			n.ingressBuffer.Set(diodes.GenericDataType(envelope))
			n.metrics.Inc(debug.NozzleIngressEnvelopesTotal)
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

func (n *Nozzle) timerEmitter(ch chan []*rpc.Point) {
	ticker := time.NewTicker(n.rollupInterval)

	for t := range ticker.C {
		timestampNano := t.Truncate(n.rollupInterval).UnixNano()

		var size int
		var points []*rpc.Point

		for _, pointsBatch := range n.totalRollup.Rollup(timestampNano) {
			points = append(points, pointsBatch.Points...)
			size += pointsBatch.Size

			if size >= ingressclient.MAX_BATCH_SIZE_IN_BYTES {
				points = n.writeToChannelOrDiscard(ch, points)
				size = 0
			}
		}

		for _, pointsBatch := range n.durationRollup.Rollup(timestampNano) {
			points = append(points, pointsBatch.Points...)
			size += pointsBatch.Size

			if size >= ingressclient.MAX_BATCH_SIZE_IN_BYTES {
				points = n.writeToChannelOrDiscard(ch, points)
				size = 0
			}
		}

		if len(points) > 0 {
			points = n.writeToChannelOrDiscard(ch, points)
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

func (n *Nozzle) convertEnvelopeToPoints(envelope *loggregator_v2.Envelope) []*rpc.Point {
	switch envelope.Message.(type) {
	case *loggregator_v2.Envelope_Gauge:
		return n.createPointsFromGauge(envelope)
	case *loggregator_v2.Envelope_Timer:
		timer := envelope.GetTimer()
		if timer.GetName() != n.rollupMetricName {
			return []*rpc.Point{}
		}

		if strings.ToLower(envelope.Tags["peer_type"]) == "client" {
			return []*rpc.Point{}
		}

		n.timerBuffer.Set(diodes.GenericDataType(envelope))
	case *loggregator_v2.Envelope_Counter:
		return []*rpc.Point{n.createPointFromCounter(envelope)}
	}

	return []*rpc.Point{}
}

func (n *Nozzle) createPointsFromGauge(envelope *loggregator_v2.Envelope) []*rpc.Point {
	var points []*rpc.Point
	gauge := envelope.GetGauge()
	for name, metric := range gauge.GetMetrics() {
		labels := map[string]string{
			"source_id": envelope.GetSourceId(),
			"unit":      metric.GetUnit(),
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
