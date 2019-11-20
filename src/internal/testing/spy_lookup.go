package testing

import "sync"

type SpyLookup struct {
	mu       sync.Mutex
	HashKeys []string
	Results  map[string][]int
}

func NewSpyLookup() *SpyLookup {
	return &SpyLookup{
		Results: make(map[string][]int),
	}
}

func (s *SpyLookup) Lookup(hashKey string) []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.HashKeys = append(s.HashKeys, hashKey)
	return s.Results[hashKey]
}

func (s *SpyLookup) GetHashKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.HashKeys
}
