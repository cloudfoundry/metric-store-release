package metricstore

import (
	"net/url"

	"github.com/prometheus/prometheus/rules"
	"github.com/prometheus/prometheus/scrape"
	tsdbLabels "github.com/prometheus/tsdb/labels"
)

type nullTargetRetriever struct{}

func (tr *nullTargetRetriever) TargetsActive() map[string][]*scrape.Target  { return nil }
func (tr *nullTargetRetriever) TargetsDropped() map[string][]*scrape.Target { return nil }

type nullAlertmanagerRetriever struct{}

func (ar *nullAlertmanagerRetriever) Alertmanagers() []*url.URL        { return nil }
func (ar *nullAlertmanagerRetriever) DroppedAlertmanagers() []*url.URL { return nil }

type nullRulesRetriever struct{}

func (rr *nullRulesRetriever) RuleGroups() []*rules.Group           { return nil }
func (rr *nullRulesRetriever) AlertingRules() []*rules.AlertingRule { return nil }

type nullTSDBAdmin struct{}

func (a *nullTSDBAdmin) CleanTombstones() error                           { return nil }
func (a *nullTSDBAdmin) Delete(int64, int64, ...tsdbLabels.Matcher) error { return nil }
func (a *nullTSDBAdmin) Dir() string                                      { return "" }
func (a *nullTSDBAdmin) Snapshot(string, bool) error                      { return nil }
