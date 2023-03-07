package query

import "github.com/prometheus/prometheus/model/labels"

type LabeledIterator struct {
	Iterator Iterator
	Labels   labels.Labels
}
