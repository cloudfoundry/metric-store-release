package routing

import (
	"errors"

	"github.com/benbjohnson/jmphash"
	"github.com/cespare/xxhash"
)

// RoutingTable makes decisions for where a item should be routed.
type RoutingTable struct {
	addresses         map[string]int
	replicationFactor uint
	hasher            *jmphash.Hasher
	table             []hostRange
}

// NewRoutingTable returns a new RoutingTable.
func NewRoutingTable(addrs []string, replicationFactor uint) (*RoutingTable, error) {
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
		result = append(result, (node+n*int(t.replicationFactor))%len(t.addresses))
	}
	for _, r := range t.table {
		if hashValue >= r.start && hashValue <= r.end {
			result = append(result, r.hostIndex)
		}
	}

	return result
}

type rangeInfo struct {
	start uint64
	end   uint64
}

type hostRange struct {
	hostIndex int
	rangeInfo
}
