package routing

import (
	"log"

	"github.com/emirpasic/gods/trees/avltree"
	"github.com/emirpasic/gods/utils"
)

// StaticLookup is used to do lookup for static routes.
type StaticLookup struct {
	hash func(string) uint64
	t    *avltree.Tree
}

// NewStaticLookup creates and returns a StaticLookup.
func NewStaticLookup(numOfRoutes int, hasher func(string) uint64) *StaticLookup {
	if numOfRoutes <= 0 {
		log.Panicf("Invalid number of routes: %d", numOfRoutes)
	}

	t := avltree.NewWith(utils.UInt64Comparator)

	// NOTE: 18446744073709551615 is 0xFFFFFFFFFFFFFFFF or the max value of a
	// uint64.
	x := (18446744073709551615 / uint64(numOfRoutes))
	for i := uint64(0); i < uint64(numOfRoutes); i++ {
		t.Put(i*x, i)
	}

	return &StaticLookup{
		hash: hasher,
		t:    t,
	}
}

// Lookup hashes the SourceId and then returns the index that is in range of
// the hash.
func (l *StaticLookup) Lookup(sourceId string) int {
	h := l.hash(sourceId)
	n, _ := l.t.Floor(h)
	return int(n.Value.(uint64))
}
