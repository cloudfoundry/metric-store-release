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

func (p *Point) EstimatePointSize() (size int) {
	size += len(p.Name)

	// 8 bytes for timestamp (int64), 8 bytes for value (float64)
	size += 16

	// add the size of all label keys and values
	for k, v := range p.Labels {
		size += (len(k) + len(v))
	}

	return size
}
