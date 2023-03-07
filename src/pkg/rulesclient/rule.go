package rulesclient

import (
	"errors"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3"
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
		promError := rulefmt.Error{Err: errs[0]}
		return errors.New(promError.Error())
	}

	return nil
}

func (r *Rule) convertToPromRule() (rulefmt.RuleNode, error) {
	var duration time.Duration
	var err error
	if r.For != "" {
		duration, err = time.ParseDuration(strings.Trim(r.For, `"`))
		if err != nil {
			return rulefmt.RuleNode{}, err
		}
	}

	record := YAMLNodeFromString(r.Record)
	alert := YAMLNodeFromString(r.Alert)
	expr := YAMLNodeFromString(r.Expr)

	return rulefmt.RuleNode{
		Record:      record,
		Alert:       alert,
		Expr:        expr,
		For:         model.Duration(duration),
		Labels:      r.Labels,
		Annotations: r.Annotations,
	}, nil
}

func YAMLNodeFromString(value string) yaml.Node {
	node := yaml.Node{}
	if value != "" {
		node.SetString(value)
	}
	return node
}
