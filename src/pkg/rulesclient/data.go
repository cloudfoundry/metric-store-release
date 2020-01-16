package rulesclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
)

type ApiErrors struct {
	Errors []ApiError `json:"errors"`
}

type ApiError struct {
	Status int    `json:"status"`
	Title  string `json:"title"`
}

type Manager struct {
	Id              string `json:"id"`
	AlertManagerUrl string `json:"alertmanager_url"`
}

type ManagerData struct {
	Data Manager `json:"data"`
}

type Rule struct {
	Record      string            `json:"record,omitempty"`
	Alert       string            `json:"alert,omitempty"`
	Expr        string            `json:"expr"`
	For         string            `json:"for,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
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

type RuleGroup struct {
	Name     string   `json:"name"`
	Interval Duration `json:"interval,omitempty"`
	Rules    []Rule   `json:"rules"`
}

func (rg *RuleGroup) Validate() error {
	if rg.Name == "" {
		return errors.New("name is required")
	}

	if len(rg.Rules) == 0 {
		return errors.New("at least one rule is required")
	}

	for _, rule := range rg.Rules {
		promRule, err := rule.convertToPromRule()
		if err != nil {
			return err
		}
		errs := promRule.Validate()
		if len(errs) != 0 {
			return errs[0]
		}
	}

	return nil
}

func (rg *RuleGroup) ConvertToPromRuleGroup() (*rulefmt.RuleGroup, error) {
	var promRules []rulefmt.Rule
	for _, rule := range rg.Rules {
		promRule, err := rule.convertToPromRule()
		if err != nil {
			return nil, err
		}

		promRules = append(promRules, promRule)
	}

	return &rulefmt.RuleGroup{
		Name:     rg.Name,
		Interval: model.Duration(rg.Interval),
		Rules:    promRules,
	}, nil
}

type RuleGroupData struct {
	Data RuleGroup `json:"data"`
}

type Duration time.Duration

func (d *Duration) Type() string {
	return "duration"
}

func (d Duration) String() string {
	var (
		ms   = int64(time.Duration(d) / time.Millisecond)
		unit = "ms"
	)
	if ms == 0 {
		return "0s"
	}
	factors := map[string]int64{
		"y":  1000 * 60 * 60 * 24 * 365,
		"w":  1000 * 60 * 60 * 24 * 7,
		"d":  1000 * 60 * 60 * 24,
		"h":  1000 * 60 * 60,
		"m":  1000 * 60,
		"s":  1000,
		"ms": 1,
	}

	switch int64(0) {
	case ms % factors["y"]:
		unit = "y"
	case ms % factors["w"]:
		unit = "w"
	case ms % factors["d"]:
		unit = "d"
	case ms % factors["h"]:
		unit = "h"
	case ms % factors["m"]:
		unit = "m"
	case ms % factors["s"]:
		unit = "s"
	}
	return fmt.Sprintf("%v%v", ms/factors[unit], unit)
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		dur := time.Duration(value)
		*d = Duration(dur)
		return nil
	case string:
		var err error
		dur, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(dur)
		return nil
	default:
		return errors.New("invalid duration")
	}
}
