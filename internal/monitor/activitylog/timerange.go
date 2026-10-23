package activitylog

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var offsetRe = regexp.MustCompile(`(?i)^(?:(\d+)d)?(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)

// parseOffset parses Python az's ##d##h##m##s period format.
func parseOffset(s string) (time.Duration, error) {
	m := offsetRe.FindStringSubmatch(s)
	if s == "" || m == nil {
		return 0, fmt.Errorf("invalid --offset %q, expected a format like 30d, 6h or 1d12h", s)
	}
	var d time.Duration
	for i, unit := range []time.Duration{24 * time.Hour, time.Hour, time.Minute, time.Second} {
		if m[i+1] != "" {
			n, _ := strconv.Atoi(m[i+1])
			d += time.Duration(n) * unit
		}
	}
	return d, nil
}

func parseTime(flag, s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid --%s %q, expected ISO8601 e.g. 2024-01-01T00:00:00Z", flag, s)
}

// timeRange fills in missing bounds from the offset, like Python az.
func timeRange(startTime, endTime, offset string, now time.Time) (time.Time, time.Time, error) {
	d, err := parseOffset(offset)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	var start, end time.Time
	if startTime != "" {
		if start, err = parseTime("start-time", startTime); err != nil {
			return start, end, err
		}
	}
	if endTime != "" {
		if end, err = parseTime("end-time", endTime); err != nil {
			return start, end, err
		}
	}
	switch {
	case startTime != "" && endTime != "":
	case startTime != "":
		end = start.Add(d)
	case endTime != "":
		start = end.Add(-d)
	default:
		end = now
		start = now.Add(-d)
	}
	return start.UTC(), end.UTC(), nil
}
