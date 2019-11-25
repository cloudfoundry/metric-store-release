package matchers

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/onsi/gomega/types"
)

func ContainPoint(expected interface{}) types.GomegaMatcher {
	return &containPointsMatcher{
		expected: []*rpc.Point{expected.(*rpc.Point)},
	}
}
