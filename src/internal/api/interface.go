package api

import (
	"github.com/prometheus/prometheus/scrape"
	tsdbLabels "github.com/prometheus/prometheus/tsdb/labels"
)

type nullTargetRetriever struct{}

func (tr *nullTargetRetriever) TargetsActive() map[string][]*scrape.Target  { return nil }
func (tr *nullTargetRetriever) TargetsDropped() map[string][]*scrape.Target { return nil }

type nullTSDBAdmin struct{}

func (a *nullTSDBAdmin) CleanTombstones() error                           { return nil }
func (a *nullTSDBAdmin) Delete(int64, int64, ...tsdbLabels.Matcher) error { return nil }
func (a *nullTSDBAdmin) Dir() string                                      { return "" }
func (a *nullTSDBAdmin) Snapshot(string, bool) error                      { return nil }
