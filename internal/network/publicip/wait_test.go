package publicip

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
		{"exists and present", true, "Updating", false, true, true},
		{"exists but absent", false, "", false, true, false},
		{"default and succeeded", true, "Succeeded", false, false, true},
		{"default but updating", true, "Updating", false, false, false},
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
