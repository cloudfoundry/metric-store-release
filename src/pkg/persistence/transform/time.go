package transform

import (
	"strconv"
	"time"
)

func SecondsToMilliseconds(s int64) int64 {
	return (s * int64(time.Second)) / int64(time.Millisecond)
}

func MillisecondsToNanoseconds(ms int64) int64 {
	return (ms * int64(time.Millisecond)) / int64(time.Nanosecond)
}

func MillisecondsToTime(ms int64) time.Time {
	return time.Unix(0, MillisecondsToNanoseconds(ms))
}

func MillisecondsToString(ms int64) string {
	return strconv.FormatFloat(float64(ms)/1000.0, 'f', -1, 64)
}

func NanosecondsToMilliseconds(ns int64) int64 {
	return (ns * int64(time.Nanosecond)) / int64(time.Millisecond)
}

func DurationToSeconds(t time.Duration) float64 {
	return float64(t) / float64(time.Second)
}
