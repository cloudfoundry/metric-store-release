package api

import (
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
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
		duration, err = time.ParseDuration(strings.Trim(string(r.For), `"`))
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
