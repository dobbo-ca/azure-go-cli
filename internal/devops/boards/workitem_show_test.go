package boards

import (
	"strings"
	"testing"
)

// TestWorkitemParseAsOfUTCOffset covers convert_date_string_to_iso8601
// (common/arguments.py:15-28): Python's datetime.isoformat() always emits a
// numeric UTC offset ("+00:00"), never a bare "Z", even for a UTC input.
func TestWorkitemParseAsOfUTCOffset(t *testing.T) {
	got, err := workitemParseAsOf("2024-01-02 15:04:05 UTC")
	if err != nil {
		t.Fatalf("workitemParseAsOf: %v", err)
	}
	if strings.HasSuffix(got, "Z") {
		t.Errorf("workitemParseAsOf = %q, want a numeric offset (+00:00), not a bare Z", got)
	}
	if !strings.HasSuffix(got, "+00:00") {
		t.Errorf("workitemParseAsOf = %q, want it to end in +00:00", got)
	}
}
