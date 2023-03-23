package api

import (
	"strings"
	"time"

	"github.com/prometheus/common/model"
	prom_labels "github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/rulefmt"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/tsdb"
)

type nullTargetRetriever struct{}

func (tr *nullTargetRetriever) TargetsActive() map[string][]*scrape.Target  { return nil }
func (tr *nullTargetRetriever) TargetsDropped() map[string][]*scrape.Target { return nil }

type nullTSDBAdminStats struct{}

func (a *nullTSDBAdminStats) WALReplayStatus() (tsdb.WALReplayStatus, error) {
	return tsdb.WALReplayStatus{}, nil
}
func (a *nullTSDBAdminStats) CleanTombstones() error                             { return nil }
func (a *nullTSDBAdminStats) Delete(int64, int64, ...*prom_labels.Matcher) error { return nil }
func (a *nullTSDBAdminStats) Snapshot(string, bool) error                        { return nil }
func (a *nullTSDBAdminStats) Stats(string) (*tsdb.Stats, error)                  { return nil, nil }

type Rule struct {
	Record      string            `yaml:"record,omitempty"`
	Alert       string            `yaml:"alert,omitempty"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

func (r *Rule) convertToPromRule() (rulefmt.Rule, error) {
	var duration time.Duration
	var err error
	if r.For != "" {
		duration, err = time.ParseDuration(strings.Trim(r.For, `"`))
		if err != nil {
			return rulefmt.Rule{}, err
		}
	}

	return rulefmt.Rule{
		Record:      r.Record,
		Alert:       r.Alert,
		Expr:        r.Expr,
		For:         model.Duration(duration),
		Labels:      r.Labels,
		Annotations: r.Annotations,
	}, nil
}
