package transform

import (
	"regexp"
	"time"

	rpc "github.com/cloudfoundry/metric-store/src/pkg/rpc/metricstore_v1"
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

func SanitizeMetricName(name string) string {
	// Forcefully convert all invalid separators to underscores
	// First character: Match if it's NOT A-z, underscore or colon [^A-z_:]
	// All others: Match if they're NOT alphanumeric, understore, or colon [\W_:]+?

	var re = regexp.MustCompile(`^[^A-z_:]|[^\w_:]+?`)
	return re.ReplaceAllString(name, "_")
}

func SanitizeLabelName(name string) string {
	// Forcefully convert all invalid separators to underscores
	// First character: Match if it's NOT A-z, underscore or colon [^A-z_:]
	// All others: Match if they're NOT alphanumeric, understore, or colon [\W_:]+?

	var re = regexp.MustCompile(`^[^A-z_:]|[^\w_]+?`)
	return re.ReplaceAllString(name, "_")
}
