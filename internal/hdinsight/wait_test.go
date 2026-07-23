package hdinsight

import "testing"

func TestWaitDone(t *testing.T) {
	tests := []struct {
		name    string
		found   bool
		state   string
		deleted bool
		exists  bool
		want    bool
	}{
		{"deleted and gone", false, "", true, false, true},
		{"deleted but present", true, "Succeeded", true, false, false},
		{"exists and present", true, "InProgress", false, true, true},
		{"exists but absent", false, "", false, true, false},
		{"default and succeeded", true, "Succeeded", false, false, true},
		{"default but in progress", true, "InProgress", false, false, false},
		{"default but absent", false, "", false, false, false},
		{"succeeded case-insensitive", true, "succeeded", false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := waitDone(tt.found, tt.state, tt.deleted, tt.exists); got != tt.want {
				t.Errorf("waitDone(%v, %q, deleted=%v, exists=%v) = %v, want %v", tt.found, tt.state, tt.deleted, tt.exists, got, tt.want)
			}
		})
	}
}

func TestParseTags(t *testing.T) {
	got := parseTags([]string{"env=prod", "team=data", "bare"})
	if len(got) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(got))
	}
	if got["env"] == nil || *got["env"] != "prod" {
		t.Errorf("env tag = %v, want prod", got["env"])
	}
	if got["bare"] == nil || *got["bare"] != "" {
		t.Errorf("bare tag = %v, want empty", got["bare"])
	}
	if parseTags(nil) != nil {
		t.Errorf("parseTags(nil) should be nil")
	}
}
