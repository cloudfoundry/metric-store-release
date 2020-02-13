package storage

type Routing interface {
	Lookup(item string) []int
	IsLocal(metricName string) bool
}
