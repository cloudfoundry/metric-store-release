package routing

import (
	"math/rand"
)

// TODO: do we even need this file anymore?
type Clients struct {
	lookup     Lookup
	localIndex int
}

type Lookup func(hashKey string) []int

func NewClients(lookup Lookup, localIndex int) *Clients {
	return &Clients{
		lookup:     lookup,
		localIndex: localIndex,
	}
}

func (clients *Clients) MetricDistribution(metricName string) ([]int, bool) {
	var metricContainedLocally bool

	clientIndexes := clients.lookup(metricName)
	for _, clientIndex := range clientIndexes {
		if clientIndex == clients.localIndex {
			metricContainedLocally = true
		}
	}

	return clientIndexes, metricContainedLocally
}

func Shuffle(elements []int) {
	if len(elements) > 1 {
		rand.Shuffle(len(elements), func(i, j int) {
			elements[i], elements[j] = elements[j], elements[i]
		})
	}
}
