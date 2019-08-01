package metricstore

import (
	"net/url"

	"github.com/prometheus/prometheus/rules"
	"github.com/prometheus/prometheus/scrape"
	tsdbLabels "github.com/prometheus/tsdb/labels"
)

type NullTargetRetriever struct{}

func (tr *NullTargetRetriever) TargetsActive() map[string][]*scrape.Target  { return nil }
func (tr *NullTargetRetriever) TargetsDropped() map[string][]*scrape.Target { return nil }

type NullAlertmanagerRetriever struct{}

func (ar *NullAlertmanagerRetriever) Alertmanagers() []*url.URL        { return nil }
func (ar *NullAlertmanagerRetriever) DroppedAlertmanagers() []*url.URL { return nil }

type NullRulesRetriever struct{}

func (rr *NullRulesRetriever) RuleGroups() []*rules.Group           { return nil }
func (rr *NullRulesRetriever) AlertingRules() []*rules.AlertingRule { return nil }

type NullTSDBAdmin struct{}

func (a *NullTSDBAdmin) CleanTombstones() error                           { return nil }
func (a *NullTSDBAdmin) Delete(int64, int64, ...tsdbLabels.Matcher) error { return nil }
func (a *NullTSDBAdmin) Dir() string                                      { return "" }
func (a *NullTSDBAdmin) Snapshot(string, bool) error                      { return nil }
