package rulesclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type RuleGroupData struct {
	Data RuleGroup `json:"data"`
}

type RuleGroups struct {
	Groups []RuleGroup `json:"groups"`
}

type RuleGroup struct {
	Name     string   `json:"name"`
	Interval Duration `json:"interval,omitempty"`
	Rules    []Rule   `json:"rules"`
}

type Duration time.Duration

func (rg *RuleGroup) Validate() error {
	if rg.Name == "" {
		return errors.New("name is required")
	}

	if rg.Interval != 0 && rg.Interval < Duration(time.Minute) {
		return errors.New("interval is too short")
	}

	if len(rg.Rules) == 0 {
		return errors.New("at least one rule is required")
	}

	for _, rule := range rg.Rules {
		err := rule.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

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
