package routing_test

import (
	"fmt"

	"github.com/cloudfoundry/metric-store-release/src/internal/routing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
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
		r, err := routing.NewRoutingTable(0, []string{"10.0.1.1", "10.0.1.2", "10.0.1.3", "10.0.1.4"}, 3)
		Expect(err).ToNot(HaveOccurred())

		Expect(r.Lookup("200")).To(ConsistOf(3, 0, 1))
		Expect(r.Lookup("400")).To(ConsistOf(0, 1, 2))
	})

	DescribeTable("Returns the correct nodes",
		func(item string, numNodes, replicationFactor int, nodes []int) {
			var addrs []string
			for i := 0; i < numNodes; i++ {
				addrs = append(addrs, fmt.Sprintf("10.0.1.%d", i))
			}
			r, err := routing.NewRoutingTable(0, addrs, uint(replicationFactor))
			Expect(err).ToNot(HaveOccurred())
			Expect(r.Lookup(item)).To(ConsistOf(nodes))
		},
		Entry("1 node, RF 1, a", "200", 1, 1, []int{0}),
		Entry("1 node, RF 1, b", "400", 1, 1, []int{0}),
		Entry("2 nodes, RF 1, a", "200", 2, 1, []int{0}),
		Entry("2 nodes, RF 1, b", "400", 2, 1, []int{0}),
		Entry("2 nodes, RF 2, a", "200", 2, 2, []int{0, 1}),
		Entry("2 nodes, RF 2, b", "400", 2, 2, []int{0, 1}),
		Entry("3 nodes, RF 1, a", "200", 3, 1, []int{0}),
		Entry("3 nodes, RF 1, b", "400", 3, 1, []int{0}),
		Entry("3 nodes, RF 2, a", "200", 3, 2, []int{0, 1}),
		Entry("3 nodes, RF 2, b", "400", 3, 2, []int{0, 1}),
		Entry("3 nodes, RF 3, a", "200", 3, 3, []int{0, 1, 2}),
		Entry("3 nodes, RF 3, b", "400", 3, 3, []int{0, 1, 2}),
		Entry("6 nodes, RF 1, a", "200", 6, 1, []int{3}),
		Entry("6 nodes, RF 1, b", "400", 6, 1, []int{5}),
		Entry("6 nodes, RF 2, a", "200", 6, 2, []int{3, 4}),
		Entry("6 nodes, RF 2, b", "400", 6, 2, []int{5, 0}),
		Entry("6 nodes, RF 3, a", "200", 6, 3, []int{3, 4, 5}),
		Entry("6 nodes, RF 3, b", "400", 6, 3, []int{5, 0, 1}),
		Entry("6 nodes, RF 4, a", "200", 6, 4, []int{3, 4, 5, 0}),
		Entry("6 nodes, RF 4, b", "400", 6, 4, []int{5, 0, 1, 2}),
		Entry("6 nodes, RF 5, a", "200", 6, 5, []int{3, 4, 5, 0, 1}),
		Entry("6 nodes, RF 5, b", "400", 6, 5, []int{5, 0, 1, 2, 3}),
		Entry("6 nodes, RF 6, a", "200", 6, 6, []int{3, 4, 5, 0, 1, 2}),
		Entry("6 nodes, RF 6, b", "400", 6, 6, []int{0, 1, 2, 3, 4, 5}),
	)

	DescribeTable("Enforces a sane replicationFactor",
		func(numNodes, replicationFactor int) {
			var addrs []string
			for i := 0; i < numNodes; i++ {
				addrs = append(addrs, fmt.Sprintf("10.0.1.%d", i))
			}
			_, err := routing.NewRoutingTable(0, addrs, uint(replicationFactor))
			Expect(err).To(HaveOccurred())
		},
		Entry("one node 0", 1, 0),
		Entry("one node 2", 1, 2),
		Entry("two nodes 0", 2, 0),
		Entry("two nodes 3", 2, 3),
		Entry("three nodes 0", 3, 0),
		Entry("three nodes 4", 3, 4),
		Entry("six nodes 0", 6, 0),
		Entry("six nodes 7", 6, 7),
	)

})
