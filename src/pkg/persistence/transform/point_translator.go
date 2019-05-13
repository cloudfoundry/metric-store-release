package transform

import (
	"math"
	"time"

	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/influxdata/influxdb/models"
	"github.com/influxdata/influxdb/query"
)

const (
	MEASUREMENT_NAME = "__name__"
)

var UNINDEXED_LABELS = map[string]bool{
	"uri":            true,
	"content_length": true,
	"user_agent":     true,
	"request_id":     true,
	"forwarded":      true,
	"remote_address": true,
}

func ToInfluxPoints(points []*rpc.Point) []models.Point {
	transformedPoints := make([]models.Point, len(points))

	for i, point := range points {
		timestamp := time.Unix(0, point.GetTimestamp())
		tags := models.NewTags(map[string]string{})

		fields := models.Fields{
			"value": point.GetValue(),
		}

		for labelName, labelValue := range point.Labels {
			if UNINDEXED_LABELS[labelName] {
				fields[labelName] = labelValue
			} else {
				tags.Set([]byte(labelName), []byte(labelValue))
			}
		}

		tsPoint := models.MustNewPoint(point.GetName(), tags, fields, timestamp)
		transformedPoints[i] = tsPoint
	}

	return transformedPoints
}

func SeriesDataFromInfluxPoint(influxPoint *query.FloatPoint, fields []string) (seriesSample, map[string]string) {
	labels := make(map[string]string)
	for key, value := range influxPoint.Tags.KeyValues() {
		labels[key] = value
	}
	for i, field := range fields {
		fieldValue, ok := influxPoint.Aux[i].(string)
		if ok {
			labels[field] = fieldValue
		}
	}
	labels[MEASUREMENT_NAME] = influxPoint.Name

	sample := seriesSample{
		TimeInMilliseconds: NanosecondsToMilliseconds(influxPoint.Time),
		Value:              influxPoint.Value,
	}
	return sample, labels
}

func SeriesDataFromPromQLSample(promQLSample *rpc.PromQL_Sample) (seriesSample, map[string]string) {
	labels := promQLSample.GetMetric()

	point := promQLSample.GetPoint()
	sample := seriesSample{
		TimeInMilliseconds: point.GetTime(),
		Value:              point.GetValue(),
	}

	return sample, labels
}

func SeriesDataFromPromQLSeries(promQLSeries *rpc.PromQL_Series) ([]seriesSample, map[string]string) {
	labels := promQLSeries.GetMetric()

	var samples []seriesSample
	points := promQLSeries.GetPoints()
	for _, point := range points {
		samples = append(samples, seriesSample{
			TimeInMilliseconds: point.GetTime(),
			Value:              point.GetValue(),
		})
	}

	return samples, labels
}

// PromQL Metric Name Sanitization
// https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
// First character: Match if it's NOT A-z, underscore, or colon [^A-z_:]
// All others: Match if they're NOT alphanumeric, underscore, or colon [\w_:]+?
func SanitizeMetricName(name string) string {
	buffer := make([]byte, len(name))

	for n := 0; n < len(name); n++ {
		if name[n] == '_' || name[n] == ':' || (name[n] >= 'A' && name[n] <= 'Z') || (name[n] >= 'a' && name[n] <= 'z') {
			buffer[n] = name[n]
			continue
		}

		if (n > 0) && (name[n] >= '0' && name[n] <= '9') {
			buffer[n] = name[n]
			continue
		}

		buffer[n] = '_'
	}

	return string(buffer)
}

// PromQL Label Name Sanitization
// From: https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
// First character: Match if it's NOT A-z or underscore [^A-z_]
// All others: Match if they're NOT alphanumeric or underscore [\w_]+?
func SanitizeLabelName(name string) string {
	buffer := make([]byte, len(name))

	for n := 0; n < len(name); n++ {
		if name[n] == '_' || (name[n] >= 'A' && name[n] <= 'Z') || (name[n] >= 'a' && name[n] <= 'z') {
			buffer[n] = name[n]
			continue
		}

		if (n > 0) && (name[n] >= '0' && name[n] <= '9') {
			buffer[n] = name[n]
			continue
		}

		buffer[n] = '_'
	}

	return string(buffer)
}

func IsValidFloat(value float64) bool {
	return !math.IsInf(value, 0) && !math.IsNaN(value)
}
