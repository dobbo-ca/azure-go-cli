package activitylog

import (
	"testing"
	"time"
)

func TestTimeRange(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		start, end, offset string
		wantStart, wantEnd time.Time
	}{
		{"", "", "30d", now.Add(-30 * 24 * time.Hour), now},
		{"", "", "6h", now.Add(-6 * time.Hour), now},
		{"2026-10-01T00:00:00Z", "", "1d12h", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)},
		{"", "2026-10-02", "1D", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)},
		{"2026-10-01", "2026-10-05", "6h", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		s, e, err := timeRange(c.start, c.end, c.offset, now)
		if err != nil || !s.Equal(c.wantStart) || !e.Equal(c.wantEnd) {
			t.Errorf("timeRange(%q, %q, %q) = %v, %v, %v", c.start, c.end, c.offset, s, e, err)
		}
	}
	for _, bad := range []string{"", "30", "abc", "1h1d"} {
		if _, _, err := timeRange("", "", bad, now); err == nil {
			t.Errorf("expected error for offset %q", bad)
		}
	}
}
