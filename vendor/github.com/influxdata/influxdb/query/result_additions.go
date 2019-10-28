package query

import "github.com/prometheus/prometheus/pkg/labels"

type LabeledIterator struct {
	Iterator Iterator
	Labels   labels.Labels
}
