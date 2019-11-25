package testing

import (
	"fmt"
	"time"
)

func FormatTimeWithDecimalMillis(t time.Time) string {
	return fmt.Sprintf("%.3f", float64(t.UnixNano())/1e9)
}
