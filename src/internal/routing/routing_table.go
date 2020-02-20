package routing

import (
	"errors"
	"math/rand"

	"github.com/benbjohnson/jmphash"
	"github.com/cespare/xxhash"
)

type Lookup func(hashKey string) []int

// RoutingTable makes decisions for where a item should be routed.
type RoutingTable struct {
	localNode         int
	addresses         map[string]int
	replicationFactor uint
	hasher            *jmphash.Hasher
	table             []hostRange
}

// NewRoutingTable returns a new RoutingTable.
func NewRoutingTable(localNode int, addrs []string, replicationFactor uint) (*RoutingTable, error) {
	if replicationFactor == 0 {
		return nil, errors.New("replication factor must be greater than 0")
	}

	if replicationFactor > uint(len(addrs)) {
		return nil, errors.New("replication factor cannot exceed number of available hosts")
	}

	addresses := make(map[string]int)
	for i, addr := range addrs {
		addresses[addr] = i
	}

	t := &RoutingTable{
		localNode:         localNode,
		addresses:         addresses,
		replicationFactor: replicationFactor,
		hasher:            jmphash.NewHasher(len(addrs)),
	}

	return t, nil
}

// Lookup takes a item, hash it and determine what node(s) it should be
// routed to.
func (t *RoutingTable) Lookup(item string) []int {
	hashValue := xxhash.Sum64String(item)
	node := t.hasher.Hash(hashValue)

	var result []int
	for n := 0; n < int(t.replicationFactor); n++ {
		result = append(result, (node+n)%len(t.addresses))
	}
	for _, r := range t.table {
		if hashValue >= r.start && hashValue <= r.end {
			result = append(result, r.hostIndex)
		}
	}
	return result
}

func (t *RoutingTable) IsLocal(metricName string) bool {
	clientIndexes := t.Lookup(metricName)
	for _, clientIndex := range clientIndexes {
		if clientIndex == t.localNode {
			return true
		}
	}

	return false
}

func Shuffle(elements []int) {
	if len(elements) > 1 {
		rand.Shuffle(len(elements), func(i, j int) {
			elements[i], elements[j] = elements[j], elements[i]
		})
	}
}

type rangeInfo struct {
	start uint64
	end   uint64
}

type hostRange struct {
	hostIndex int
	rangeInfo
}
