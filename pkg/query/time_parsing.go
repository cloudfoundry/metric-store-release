package query

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/metric-store/pkg/persistence/transform"
	"github.com/prometheus/common/model"
)

func ParseStep(param string) (time.Duration, error) {
	if step, err := strconv.ParseFloat(param, 64); err == nil {
		stepInNanoSeconds := step * float64(time.Second)
		if stepInNanoSeconds > float64(math.MaxInt64) || stepInNanoSeconds < float64(math.MinInt64) {
			return 0, fmt.Errorf("cannot parse %q to a valid step. It overflows int64", param)
		}
		return time.Duration(stepInNanoSeconds), nil
	}
	if step, err := ParseDuration(param); err == nil {
		return step, nil
	}
	return 0, fmt.Errorf("cannot parse %q to a valid step", param)
}

func ParseDuration(param string) (time.Duration, error) {
	duration, err := model.ParseDuration(param)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q to a valid duration", param)
	}

	return time.Duration(duration), nil
}

func ParseTime(param string) (time.Time, error) {
	if t, err := parseFloat(param); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, param); err == nil {
		return t, nil
	}
	return time.Unix(0, 0), fmt.Errorf("cannot parse %q to a valid Unix or RFC3339 timestamp", param)
}

func parseFloat(param string) (time.Time, error) {
	var milliseconds int64
	parts := strings.Split(param, ".")

	secondsPart, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Unix(0, 0), err
	}
	milliseconds = secondsPart * 1000

	if len(parts) > 1 {
		milliseondPart, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return time.Unix(0, 0), err
		}
		milliseconds += milliseondPart
	}

	return transform.MillisecondsToTime(milliseconds), nil
}
