package routing_test

import (
	"github.com/cloudfoundry/metric-store-release/src/internal/routing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutingTable", func() {
	It("returns the correct index for the node", func() {
		r, err := routing.NewRoutingTable(0, []string{"10.0.1.1", "10.0.1.2", "10.0.1.3", "10.0.1.4"}, 1)
		Expect(err).ToNot(HaveOccurred())

		// Explanation of hash outputs:
		// "0"   -> xxHash -> 7148434200721666028  -> jmpHash(4) -> 3
		// "200" -> xxHash -> 2685111111418367100  -> jmpHash(4) -> 3
		// "400" -> xxHash -> 11111781720710347155 -> jmpHash(4) -> 0

		Expect(r.Lookup("0")).To(ConsistOf(3))
		Expect(r.Lookup("200")).To(ConsistOf(3))
		Expect(r.Lookup("400")).To(ConsistOf(0))
	})

	It("returns the correct index with overlapping nodes", func() {
		r, err := routing.NewRoutingTable(0,[]string{"10.0.1.1", "10.0.1.2", "10.0.1.3", "10.0.1.4"}, 3)
		Expect(err).ToNot(HaveOccurred())

		Expect(r.Lookup("200")).To(ConsistOf(3, 2, 1))
		Expect(r.Lookup("400")).To(ConsistOf(0, 3, 2))
	})

	It("returns an error if replication factor is invalid", func() {
		_, err := routing.NewRoutingTable(0,[]string{"10.0.1.1", "10.0.1.2", "10.0.1.3", "10.0.1.4"}, 0)
		Expect(err).To(HaveOccurred())

		_, err = routing.NewRoutingTable(0,[]string{ "10.0.1.1", "10.0.1.2", "10.0.1.3", "10.0.1.4"}, 5)
		Expect(err).To(HaveOccurred())
	})
})
