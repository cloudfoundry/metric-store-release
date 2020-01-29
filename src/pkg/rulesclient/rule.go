package rulesclient

import (
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
)

type Rule struct {
	Record      string            `json:"record,omitempty"`
	Alert       string            `json:"alert,omitempty"`
	Expr        string            `json:"expr"`
	For         string            `json:"for,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

func (r *Rule) Validate() error {
	promRule, err := r.convertToPromRule()
	if err != nil {
		return err
	}
	errs := promRule.Validate()
	if len(errs) != 0 {
		return errs[0]
	}

	return nil
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
