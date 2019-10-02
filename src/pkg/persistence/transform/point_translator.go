package transform

import (
	"math"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/influxdata/influxdb/models"
	"github.com/influxdata/influxdb/query"
	"github.com/prometheus/prometheus/pkg/labels"
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
		timestamp := time.Unix(0, point.Timestamp)
		tags := models.NewTags(map[string]string{})

		fields := models.Fields{
			"value": point.Value,
		}

		for labelName, labelValue := range point.Labels {
			if UNINDEXED_LABELS[labelName] {
				fields[labelName] = labelValue
			} else {
				tags.Set([]byte(labelName), []byte(labelValue))
			}
		}

		tsPoint := models.MustNewPoint(point.Name, tags, fields, timestamp)
		transformedPoints[i] = tsPoint
	}

	return transformedPoints
}

func SeriesDataFromInfluxPoint(influxPoint *query.FloatPoint, fields []string) (seriesSample, labels.Labels) {
	labelBuilder := labels.NewBuilder(nil)

	for key, value := range influxPoint.Tags.KeyValues() {
		labelBuilder.Set(key, value)
	}
	for i, field := range fields {
		fieldValue, ok := influxPoint.Aux[i].(string)
		if ok {
			labelBuilder.Set(field, fieldValue)
		}
	}
	labelBuilder.Set(MEASUREMENT_NAME, influxPoint.Name)

	sample := seriesSample{
		TimeInMilliseconds: NanosecondsToMilliseconds(influxPoint.Time),
		Value:              influxPoint.Value,
	}
	return sample, labelBuilder.Labels()
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

func ConvertLabels(point *rpc.Point) labels.Labels {
	originalLabels := point.Labels
	newLabels := labels.Labels{}

	if originalLabels == nil {
		originalLabels = make(map[string]string)
	}

	for key, value := range originalLabels {
		newLabels = append(newLabels, labels.Label{Name: key, Value: value})
	}
	newLabels = append(newLabels, labels.Label{Name: "__name__", Value: point.Name})

	return newLabels
}
