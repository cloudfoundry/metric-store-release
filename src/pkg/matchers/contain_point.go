package matchers

import (
	rpc "github.com/cloudfoundry/metric-store/src/pkg/rpc/metricstore_v1"
	"github.com/onsi/gomega/types"
)

func ContainPoint(expected interface{}) types.GomegaMatcher {
	return &containPointsMatcher{
		expected: []*rpc.Point{expected.(*rpc.Point)},
	}
}
