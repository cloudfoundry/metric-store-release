package nozzle

import (
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
	"github.com/cloudfoundry/metric-store/pkg/metrics"
	rpc "github.com/cloudfoundry/metric-store/pkg/rpc/metricstore_v1"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	mapset "github.com/deckarep/golang-set"
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

	addr string
	opts []grpc.DialOption

	tagInfo *sync.Map

	rollupInterval   time.Duration
	rollupMetricName string
	rollupTags       []string
	recordTimerDrift func(float64)
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

func NewNozzle(c StreamConnector, metricStoreAddr string, shardId string, nodeIndex int, opts ...NozzleOption) *Nozzle {
	n := &Nozzle{
		s:                     c,
		addr:                  metricStoreAddr,
		opts:                  []grpc.DialOption{grpc.WithInsecure()},
		log:                   log.New(ioutil.Discard, "", 0),
		metrics:               metrics.NullMetrics{},
		shardId:               shardId,
		nodeIndex:             nodeIndex,
		timerRollupBufferSize: 4096,
		forwardedTags:         []string{},
		timerMetrics:          make(map[string]*timerValue),
		recordTimerDrift:      func(float64) {},
		tagInfo:               &sync.Map{},
	}

	for _, o := range opts {
		o(n)
	}

	n.buffer = diodes.NewOneToOne(int(n.timerRollupBufferSize), diodes.AlertFunc(func(missed int) {
		n.log.Printf("Timer buffer dropped %d points", missed)
	}))

	n.streamBuffer = diodes.NewOneToOne(100000, diodes.AlertFunc(func(missed int) {
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

// Start starts reading envelopes from the logs provider and writes them to
// metric-store. It blocks indefinitely.
func (n *Nozzle) Start() {
	rx := n.s.Stream(context.Background(), n.buildBatchReq())

	conn, err := grpc.Dial(n.addr, n.opts...)
	if err != nil {
		log.Fatalf("failed to dial %s: %s", n.addr, err)
	}
	client := rpc.NewIngressClient(conn)

	ingressInc := n.metrics.NewCounter("nozzle_ingress")
	egressInc := n.metrics.NewCounter("nozzle_egress")
	errInc := n.metrics.NewCounter("nozzle_err")
	n.recordTimerDrift = n.metrics.NewSummary("nozzle_timer_drift", "seconds")

	go n.timerProcessor()
	go n.timerEmitter(client)

	go n.envelopeReader(rx, ingressInc)

	ch := make(chan []*loggregator_v2.Envelope, BATCH_CHANNEL_SIZE)

	log.Printf("Starting %d nozzle workers...", runtime.NumCPU())
	for i := 0; i < 2*runtime.NumCPU(); i++ {
		go n.envelopeWriter(ch, client, errInc, egressInc)
	}

	// The batcher will block indefinitely.
	n.envelopeBatcher(ch)
}

func (n *Nozzle) envelopeBatcher(ch chan []*loggregator_v2.Envelope) {
	poller := diodes.NewPoller(n.streamBuffer)
	envelopes := make([]*loggregator_v2.Envelope, 0)
	t := time.NewTimer(BATCH_FLUSH_INTERVAL)
	for {
		data, found := poller.TryNext()

		if found {
			envelopes = append(envelopes, (*loggregator_v2.Envelope)(data))
		}

		select {
		case <-t.C:
			if len(envelopes) > 0 {
				select {
				case ch <- envelopes:
					envelopes = make([]*loggregator_v2.Envelope, 0)
				default:
					// if we can't write into the channel, it must be full, so
					// we probably need to drop these envelopes on the floor
					envelopes = envelopes[:0]
				}
			}
			t.Reset(BATCH_FLUSH_INTERVAL)
		default:
			if len(envelopes) >= BATCH_CHANNEL_SIZE {
				select {
				case ch <- envelopes:
					envelopes = make([]*loggregator_v2.Envelope, 0)
				default:
					envelopes = envelopes[:0]
				}
				t.Reset(BATCH_FLUSH_INTERVAL)
			}
			if !found {
				time.Sleep(time.Millisecond)
			}
		}
	}
}
func (n *Nozzle) envelopeWriter(ch chan []*loggregator_v2.Envelope, client rpc.IngressClient, errInc, egressInc func(uint64)) {
	for {
		envelopes := <-ch

		ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := client.Send(ctx, &rpc.SendRequest{
			Batch: &rpc.Points{
				Points: n.convertEnvelopesToPoints(envelopes),
			},
		})

		if err != nil {
			errInc(1)
			continue
		}

		egressInc(uint64(len(envelopes)))
	}
}

func (n *Nozzle) envelopeReader(rx loggregator.EnvelopeStream, ingressInc func(uint64)) {
	for {
		envelopeBatch := rx()
		for _, envelope := range envelopeBatch {
			n.streamBuffer.Set(diodes.GenericDataType(envelope))
			ingressInc(1)
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
		value := timer.GetStop() - timer.GetStart()

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
			n.timerMetrics[key] = &timerValue{count: 1, value: float64(value)}
			n.timerMutex.Unlock()
			continue
		}

		tv.value = (float64(tv.count)*tv.value + float64(value)) / float64(tv.count+1)
		tv.count += 1
		n.timerMutex.Unlock()
	}
}

func (n *Nozzle) timerEmitter(client rpc.IngressClient) {
	ticker := time.NewTicker(n.rollupInterval)
	nodeIndex := strconv.Itoa(n.nodeIndex)

	for t := range ticker.C {
		timestamp := t.Truncate(n.rollupInterval)

		var points []*rpc.Point

		start := time.Now()
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
				Name:      n.rollupMetricName + "_mean",
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
		}

		n.timerMetrics = make(map[string]*timerValue)
		n.timerMutex.Unlock()
		// TODO - do we still want these logs?
		n.log.Printf("Timer map cleared in %s", time.Since(start))

		n.log.Printf("Preparing to write %d points", len(points))

		ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := client.Send(ctx, &rpc.SendRequest{
			Batch: &rpc.Points{
				Points: points,
			},
		})

		if err != nil {
			n.log.Printf("failed to write points: %s", err)
			continue
		}
	}
}

func (n *Nozzle) convertEnvelopesToPoints(envelopes []*loggregator_v2.Envelope) []*rpc.Point {
	var points []*rpc.Point

	for _, envelope := range envelopes {
		// TODO: Remove this and all associated code before shipping.
		// NOTE: This function call allows us to keep an in-memory index of all
		// the tags seen for all envelopes received. It is useful for determining
		// which data should be kept inside metric store, but can take up a lot of
		// memory on PWS.
		// n.updateTagInfo(envelope, name, value)

		switch envelope.Message.(type) {
		case *loggregator_v2.Envelope_Gauge:
			points = append(points, n.createPointsFromGauge(envelope)...)
		case *loggregator_v2.Envelope_Timer:
			timer := envelope.GetTimer()
			if timer.GetName() == n.rollupMetricName {
				n.buffer.Set(diodes.GenericDataType(envelope))
				n.recordTimerDrift(float64(time.Now().UnixNano()-envelope.Timestamp) / float64(time.Second))
				continue
			}
		case *loggregator_v2.Envelope_Counter:
			points = append(points, n.createPointFromCounter(envelope))
		default:
			// drop the point here - there's nothing we can do with it.
		}
	}
	return points
}

type TagInfo struct {
	MetricNames        mapset.Set `json:"metric_names"`
	SourceIds          mapset.Set `json:"source_ids"`
	Count              uint64     `json:"count"`
	AverageValueLength float64    `json:"average_value_length"`
	ShortestValue      string     `json:"shortest_value"`
	LongestValue       string     `json:"longest_value"`
	UniqueValues       mapset.Set `json:"unique_values"`
}

func (n *Nozzle) GetTagInfo() map[string]TagInfo {
	tagInfoCopy := map[string]TagInfo{}
	n.tagInfo.Range(func(t, v interface{}) bool {
		tagName := t.(string)
		info := v.(*TagInfo)

		tagInfoCopy[tagName] = *info

		return true
	})

	return tagInfoCopy
}

func (n *Nozzle) GetTagInfoCsv() string {
	var s strings.Builder
	w := csv.NewWriter(&s)
	w.Write([]string{"tag_name", "num_metric_names", "num_source_ids", "count", "average_value_length", "shortest_value", "longest_value", "cardinality"})
	n.tagInfo.Range(func(t, v interface{}) bool {
		tagName := t.(string)
		info := v.(*TagInfo)

		w.Write([]string{
			tagName,
			strconv.Itoa(info.MetricNames.Cardinality()),
			strconv.Itoa(info.SourceIds.Cardinality()),
			strconv.Itoa(int(info.Count)),
			strconv.FormatFloat(info.AverageValueLength, 'f', -1, 64),
			info.ShortestValue,
			info.LongestValue,
			strconv.Itoa(info.UniqueValues.Cardinality()),
		})

		return true
	})

	w.Flush()

	return s.String()
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

func (n *Nozzle) createPointFromTimer(envelope *loggregator_v2.Envelope) *rpc.Point {
	timer := envelope.GetTimer()
	labels := map[string]string{
		"source_id": envelope.GetSourceId(),
	}
	for k, v := range envelope.GetTags() {
		labels[k] = v
	}
	return &rpc.Point{
		Timestamp: envelope.GetTimestamp(),
		Name:      timer.GetName(),
		Value:     float64(timer.GetStop() - timer.GetStart()),
		Labels:    labels,
	}
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

func (n *Nozzle) updateTagInfo(envelope *loggregator_v2.Envelope, name string, value float64) {
	tags := envelope.GetTags()
	for tagName, tagValue := range tags {
		value, ok := n.tagInfo.Load(tagName)
		var tagInfo *TagInfo
		if ok {
			tagInfo = value.(*TagInfo)
		} else {
			tagInfo = &TagInfo{
				MetricNames:  mapset.NewSet(),
				SourceIds:    mapset.NewSet(),
				UniqueValues: mapset.NewSet(),
			}
			n.tagInfo.Store(tagName, tagInfo)
		}

		tagInfo.MetricNames.Add(name)
		tagInfo.SourceIds.Add(envelope.GetSourceId())
		tagInfo.Count += 1
		tagInfo.UniqueValues.Add(tagValue)

		valueLengthDelta := float64(len(tagValue)) - tagInfo.AverageValueLength
		tagInfo.AverageValueLength += valueLengthDelta / float64(tagInfo.Count)

		if len(tagValue) < len(tagInfo.ShortestValue) || tagInfo.ShortestValue == "" {
			tagInfo.ShortestValue = tagValue
		}

		if len(tagValue) > len(tagInfo.LongestValue) {
			tagInfo.LongestValue = tagValue
		}
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
