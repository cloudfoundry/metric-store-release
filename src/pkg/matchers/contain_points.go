package matchers

import (
	"fmt"
	"reflect"

	rpc "github.com/cloudfoundry/metric-store/src/pkg/rpc/metricstore_v1"
	"github.com/onsi/gomega/types"
)

func ContainPoints(expected interface{}) types.GomegaMatcher {
	return &containPointsMatcher{
		expected: expected,
	}
}

type containPointsMatcher struct {
	expected interface{}
}

func (matcher *containPointsMatcher) Match(actual interface{}) (success bool, err error) {
	expectedPoints := matcher.expected.([]*rpc.Point)
	points := actual.([]*rpc.Point)
	foundPoints := make([]bool, len(expectedPoints))

	for _, point := range points {
		for n, expectedPoint := range expectedPoints {
			var matchTimestamp bool

			// if a timestamp > 0 is provided, assert on it
			if expectedPoint.Timestamp == 0 {
				matchTimestamp = (point.Timestamp > 0)
			} else {
				matchTimestamp = (point.Timestamp == expectedPoint.Timestamp)
			}

			if point.Name == expectedPoint.Name &&
				point.Value == expectedPoint.Value &&
				matchTimestamp &&
				reflect.DeepEqual(point.Labels, expectedPoint.Labels) {
				foundPoints[n] = true
				break
			}
		}
	}

	for _, found := range foundPoints {
		if !found {
			return false, nil
		}
	}

	return true, nil
}

func (matcher *containPointsMatcher) FailureMessage(actual interface{}) (message string) {
	var actualOutput string
	for _, a := range actual.([]*rpc.Point) {
		actualOutput += fmt.Sprintf("\t%#v\n", a)
	}
	var expectedOutput string
	for _, a := range matcher.expected.([]*rpc.Point) {
		expectedOutput += fmt.Sprintf("\t%#v\n", a)
	}
	return fmt.Sprintf("Expected\n%s\nto contain all the points \n%s", actualOutput, expectedOutput)
}

func (matcher *containPointsMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%#v\nnot to contain all the points \n%#v", actual, matcher.expected)
}
