package transform_test

import (
	"testing"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
)

var sanitizedResult string
var inputName = "this#Metric^Name*Foo@Bar!Baz12345"

func BenchmarkSanitizeMetricNames(b *testing.B) {
	var outputName string

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		outputName = transform.SanitizeMetricName(inputName)
	}

	sanitizedResult = outputName
}

func BenchmarkSanitizeLabelNames(b *testing.B) {
	var outputName string

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		outputName = transform.SanitizeMetricName(inputName)
	}

	sanitizedResult = outputName
}
