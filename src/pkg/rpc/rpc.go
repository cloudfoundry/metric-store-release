package rpc

type Point struct {
	Name      string
	Timestamp int64
	Value     float64
	Labels    map[string]string
}

type Batch struct {
	Points []*Point
}
