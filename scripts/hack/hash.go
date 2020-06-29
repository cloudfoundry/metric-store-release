package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/benbjohnson/jmphash"
	"github.com/cespare/xxhash"
)

var (
	replicationFactor = 2
	numberOfNodes     = 6
	hasher            *jmphash.Hasher
	table             []hostRange
)

func main() {
	hasher = jmphash.NewHasher(numberOfNodes)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		metricName := scanner.Text()
		fmt.Printf("%v # %s\n", Lookup(metricName), metricName)
	}
}

func Lookup(item string) []int {
	hashValue := xxhash.Sum64String(item)
	node := hasher.Hash(hashValue)

	var result []int
	for n := 0; n < replicationFactor; n++ {
		result = append(result, (node+n)%numberOfNodes)
	}
	for _, r := range table {
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
