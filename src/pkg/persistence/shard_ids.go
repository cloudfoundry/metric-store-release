package persistence

type ShardIDs []uint64

func (s ShardIDs) Len() int {
	return len(s)
}

func (s ShardIDs) Less(i int, j int) bool {
	return s[i] < s[j]
}

func (s ShardIDs) Swap(i int, j int) {
	s[i], s[j] = s[j], s[i]
}
