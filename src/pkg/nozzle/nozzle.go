package nozzle

import (
	"crypto/tls"
	"encoding/csv"
	"io/ioutil"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	diodes "code.cloudfoundry.org/go-diodes"
	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/gogo/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type timerValue struct {
	count uint64
	value float64
}

// Metrics registers Counter and Gauge metrics.
type MetricsInitializer interface {
	NewCounter(name string) func(delta uint64)
	NewGauge(name, unit string) func(value float64)
	NewSummary(name, unit string) func(value float64)
}

// Nozzle reads envelopes and writes points to metric-store.
type Nozzle struct {
	log                   *log.Logger
	s                     StreamConnector
	metrics               MetricsInitializer
	shardId               string
	forwardedTags         []string
	timerRollupBufferSize uint
	nodeIndex             int
	streamBuffer          *diodes.OneToOne

	buffer       *diodes.OneToOne
	timerMetrics map[string]*timerValue
	timerMutex   sync.Mutex

	addr        string
	ingressAddr string
	tlsConfig   *tls.Config
	opts        []grpc.DialOption

	tagInfo *sync.Map

	rollupInterval   time.Duration
	rollupMetricName string
	rollupTags       []string

	numEnvelopesIngressInc           func(uint64)
	numStreamDiodePointsDroppedInc   func(uint64)
	numStreamChannelPointsDroppedInc func(uint64)
	numTimerDiodePointsDroppedInc    func(uint64)
	numTimerChannelPointsDroppedInc  func(uint64)
	numBatchesEgressInc              func(uint64)
	numPointsEgressInc               func(uint64)
	egressWriteDuration              func(float64)
	egressWriteErrorInc              func(uint64)

	remoteConnection *leanstreams.TCPClient
	remoteAddress    string
	remoteCfg        *leanstreams.TCPClientConfig
}

// StreamConnector reads envelopes from the the logs provider.
type StreamConnector interface {
	// Stream creates a EnvelopeStream for the given request.
	Stream(ctx context.Context, req *loggregator_v2.EgressBatchRequest) loggregator.EnvelopeStream
}

const (
	BATCH_FLUSH_INTERVAL = 500 * time.Millisecond
	BATCH_CHANNEL_SIZE   = 512

	MAX_BATCH_SIZE_IN_BYTES           = 32 * 1024
	MAX_INGRESS_PAYLOAD_SIZE_IN_BYTES = 2 * MAX_BATCH_SIZE_IN_BYTES
)

func NewNozzle(c StreamConnector, metricStoreAddr, ingressAddr string, tlsConfig *tls.Config, shardId string, nodeIndex int, opts ...NozzleOption) *Nozzle {
	n := &Nozzle{
		s:                     c,
		addr:                  metricStoreAddr,
		ingressAddr:           ingressAddr,
		tlsConfig:             tlsConfig,
		opts:                  []grpc.DialOption{grpc.WithInsecure()},
		log:                   log.New(ioutil.Discard, "", 0),
		metrics:               metrics.NullMetrics{},
		shardId:               shardId,
		nodeIndex:             nodeIndex,
		timerRollupBufferSize: 4096,
		forwardedTags:         []string{},
		timerMetrics:          make(map[string]*timerValue),
		tagInfo:               &sync.Map{},
	}

	for _, o := range opts {
		o(n)
	}

	n.numEnvelopesIngressInc = n.metrics.NewCounter("nozzle_envelopes_ingress")
	n.numStreamDiodePointsDroppedInc = n.metrics.NewCounter("nozzle_stream_diode_points_dropped")
	n.numStreamChannelPointsDroppedInc = n.metrics.NewCounter("nozzle_stream_channel_points_dropped")
	n.numTimerDiodePointsDroppedInc = n.metrics.NewCounter("nozzle_timer_diode_points_dropped")
	n.numTimerChannelPointsDroppedInc = n.metrics.NewCounter("nozzle_timer_channel_points_dropped")
	n.numBatchesEgressInc = n.metrics.NewCounter("nozzle_batches_egress")
	n.numPointsEgressInc = n.metrics.NewCounter("nozzle_points_egress")
	n.egressWriteDuration = n.metrics.NewSummary("nozzle_egress_write_duration", "ms")
	n.egressWriteErrorInc = n.metrics.NewCounter("nozzle_egress_write_errors")

	n.buffer = diodes.NewOneToOne(int(n.timerRollupBufferSize), diodes.AlertFunc(func(missed int) {
		n.numTimerDiodePointsDroppedInc(uint64(missed))
		n.log.Printf("Timer buffer dropped %d points", missed)
	}))

	n.streamBuffer = diodes.NewOneToOne(100000, diodes.AlertFunc(func(missed int) {
		n.numStreamDiodePointsDroppedInc(uint64(missed))
		n.log.Printf("Stream buffer dropped %d points", missed)
	}))

	return n
}

type NozzleOption func(*Nozzle)

// WithNozzleLogger returns a NozzleOption that configures a nozzle's logger.
// It defaults to silent logging.
func WithNozzleLogger(l *log.Logger) NozzleOption {
	return func(n *Nozzle) {
		n.log = l
	}
}

// WithNozzleMetrics returns a NozzleOption that configures the metrics for the
// Nozzle. It will add metrics to the given map.
func WithNozzleMetrics(m MetricsInitializer) NozzleOption {
	return func(n *Nozzle) {
		n.metrics = m
	}
}

// WithNozzleDialOpts returns a NozzleOption that configures the dial options
// for dialing the metric-store. It defaults to grpc.WithInsecure().
func WithNozzleDialOpts(opts ...grpc.DialOption) NozzleOption {
	return func(n *Nozzle) {
		n.opts = opts
	}
}

func WithNozzleTimerRollupBufferSize(size uint) NozzleOption {
	return func(n *Nozzle) {
		n.timerRollupBufferSize = size
	}
}

func WithNozzleTimerRollup(interval time.Duration, metricName string, tags []string) NozzleOption {
	return func(n *Nozzle) {
		n.rollupInterval = interval
		n.rollupMetricName = metricName
		n.rollupTags = tags
	}
}

// Start() starts reading envelopes from the logs provider and writes them to
// metric-store. It blocks indefinitely.
func (n *Nozzle) Start() {
	rx := n.s.Stream(context.Background(), n.buildBatchReq())
	n.remoteAddress = n.ingressAddr

	n.remoteCfg = &leanstreams.TCPClientConfig{
		MaxMessageSize: MAX_INGRESS_PAYLOAD_SIZE_IN_BYTES,
		Address:        n.remoteAddress,
		TLSConfig:      n.tlsConfig,
	}
	for {
		rc, err := leanstreams.DialTCP(n.remoteCfg)
		if err != nil {
			// waiting for metric-store to start up
			time.Sleep(100 * time.Millisecond)
			continue
		}

		n.remoteConnection = rc
		break
	}

	ch := make(chan []*rpc.Point, BATCH_CHANNEL_SIZE)

	go n.timerProcessor()
	go n.timerEmitter(ch)
	go n.envelopeReader(rx)

	log.Printf("Starting %d nozzle workers...", 2*runtime.NumCPU())
	for i := 0; i < 2*runtime.NumCPU(); i++ {
		go n.pointWriter(ch)
	}

	// The batcher will block indefinitely.
	n.pointBatcher(ch)
}

func (n *Nozzle) pointBatcher(ch chan []*rpc.Point) {
	var size int

	poller := diodes.NewPoller(n.streamBuffer)
	points := make([]*metricstore_v1.Point, 0)

	t := time.NewTimer(BATCH_FLUSH_INTERVAL)
	for {
		data, found := poller.TryNext()

		if found {
			for _, point := range n.convertEnvelopeToPoints((*loggregator_v2.Envelope)(data)) {
				size += estimatePointSize(point)
				points = append(points, point)
			}
		}

		select {
		case <-t.C:
			if len(points) > 0 {
				points = writeToChannelOrDiscard(ch, points, n.numStreamChannelPointsDroppedInc)
			}
			t.Reset(BATCH_FLUSH_INTERVAL)
			size = 0
		default:
			// Do we care if one envelope procuces multiple points, in which a
			// subset crosses the threshold?

			// if len(points) >= BATCH_CHANNEL_SIZE {
			if size >= MAX_BATCH_SIZE_IN_BYTES {
				points = writeToChannelOrDiscard(ch, points, n.numStreamChannelPointsDroppedInc)
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

func writeToChannelOrDiscard(ch chan []*rpc.Point, points []*rpc.Point, dropped func(uint64)) []*rpc.Point {
	select {
	case ch <- points:
		return make([]*rpc.Point, 0)
	default:
		// if we can't write into the channel, it must be full, so
		// we probably need to drop these envelopes on the floor
		dropped(uint64(len(points)))
		return points[:0]
	}
}

func estimatePointSize(point *rpc.Point) (size int) {
	size += len(point.Name)

	// 8 bytes for timestamp (int64), 8 bytes for value (float64)
	size += 16

	// add the size of all label keys and values
	for k, v := range point.Labels {
		size += (len(k) + len(v))
	}

	return size
}

func (n *Nozzle) pointWriter(ch chan []*rpc.Point) {
	for {
		points := <-ch
		start := time.Now()

		payload, err := proto.Marshal(&rpc.SendRequest{
			Batch: &rpc.Points{
				Points: points,
			},
		})

		if err != nil {
			return
		}

		// TODO: consider adding back in a timeout (i.e. 3 seconds)
		bytesWritten, err := n.remoteConnection.Write(payload)
		n.log.Printf("Wrote %d of %d bytes\n", bytesWritten, len(payload))

		if err != nil {
			n.log.Printf("Error writing: %v\n", err)
			n.egressWriteErrorInc(1)

			if err = n.remoteConnection.Open(); err != nil {
				n.log.Printf("Could not reopen conn: %s\n", err)
				return
			}
		}

		n.egressWriteDuration(float64(time.Since(start) / time.Millisecond))
		n.numBatchesEgressInc(1)
		n.numPointsEgressInc(uint64(len(points)))
	}
}

func (n *Nozzle) envelopeReader(rx loggregator.EnvelopeStream) {
	for {
		envelopeBatch := rx()
		for _, envelope := range envelopeBatch {
			n.streamBuffer.Set(diodes.GenericDataType(envelope))
			n.numEnvelopesIngressInc(1)
		}
	}
}

func (n *Nozzle) timerProcessor() {
	poller := diodes.NewPoller(n.buffer)

	var count int
	for {
		count += 1
		data := poller.Next()
		envelope := *(*loggregator_v2.Envelope)(data)

		timer := envelope.GetTimer()
		value := float64(timer.GetStop()-timer.GetStart()) / float64(time.Millisecond)

		tags := []string{envelope.SourceId}

		for _, tagName := range n.rollupTags {
			tags = append(tags, envelope.Tags[tagName])
		}

		csvOutput := &strings.Builder{}
		csvWriter := csv.NewWriter(csvOutput)
		csvWriter.Write(tags)
		csvWriter.Flush()

		key := csvOutput.String()

		n.timerMutex.Lock()
		tv, found := n.timerMetrics[key]
		if !found {
			n.timerMetrics[key] = &timerValue{count: 1, value: value}
			n.timerMutex.Unlock()
			continue
		}

		tv.value = (float64(tv.count)*tv.value + value) / float64(tv.count+1)
		tv.count += 1
		n.timerMutex.Unlock()
	}
}

func (n *Nozzle) timerEmitter(ch chan []*rpc.Point) {
	ticker := time.NewTicker(n.rollupInterval)
	nodeIndex := strconv.Itoa(n.nodeIndex)

	for t := range ticker.C {
		timestamp := t.Truncate(n.rollupInterval)

		var size int
		var points []*rpc.Point

		n.timerMutex.Lock()
		for k, tv := range n.timerMetrics {
			keyParts, err := csv.NewReader(strings.NewReader(k)).Read()

			if err != nil {
				n.log.Printf("Skipping metric because key \"%s\" failed to decode: %s", k, err.Error())
				continue
			}

			// if we can't parse the key, there's probably some garbage in one
			// of the tags, so let's skip it
			if len(keyParts) != len(n.rollupTags)+1 {
				n.log.Printf("Skipping metric because key has %d parts: %+v", len(keyParts), keyParts)
				continue
			}

			meanPoint := &rpc.Point{
				Name:      n.rollupMetricName + "_mean_ms",
				Timestamp: timestamp.UnixNano(),
				Value:     tv.value,
				Labels: map[string]string{
					"source_id":  keyParts[0],
					"node_index": nodeIndex,
				},
			}
			countPoint := &rpc.Point{
				Name:      n.rollupMetricName + "_count",
				Timestamp: timestamp.UnixNano(),
				Value:     float64(tv.count),
				Labels: map[string]string{
					"source_id":  keyParts[0],
					"node_index": nodeIndex,
				},
			}

			for index, tagName := range n.rollupTags {
				if value := keyParts[index+1]; value != "" {
					meanPoint.Labels[tagName] = value
					countPoint.Labels[tagName] = value
				}
			}

			points = append(points, meanPoint, countPoint)
			size += estimatePointSize(meanPoint)
			size += estimatePointSize(countPoint)

			if size >= MAX_BATCH_SIZE_IN_BYTES {
				points = writeToChannelOrDiscard(ch, points, n.numTimerChannelPointsDroppedInc)
				size = 0
			}
		}

		if len(points) > 0 {
			points = writeToChannelOrDiscard(ch, points, n.numTimerChannelPointsDroppedInc)
		}

		n.timerMetrics = make(map[string]*timerValue)
		n.timerMutex.Unlock()
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

		if strings.ToLower(envelope.Tags["peer_type"]) == "server" {
			return []*rpc.Point{}
		}

		n.buffer.Set(diodes.GenericDataType(envelope))
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
