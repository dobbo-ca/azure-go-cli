package encryptionset

import "testing"

func TestWaitSatisfied(t *testing.T) {
	tests := []struct {
		name  string
		found bool
		state string
		o     waitOpts
		want  bool
	}{
		{"deleted and gone", false, "", waitOpts{deleted: true}, true},
		{"deleted but still present", true, "Succeeded", waitOpts{deleted: true}, false},
		{"exists and present", true, "Updating", waitOpts{exists: true}, true},
		{"exists but absent", false, "", waitOpts{exists: true}, false},
		{"created and succeeded", true, "Succeeded", waitOpts{created: true}, true},
		{"created but still updating", true, "Updating", waitOpts{created: true}, false},
		{"created but absent", false, "", waitOpts{created: true}, false},
		{"default waits for succeeded", true, "succeeded", waitOpts{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := waitSatisfied(tt.found, tt.state, tt.o); got != tt.want {
				t.Errorf("waitSatisfied(%v, %q, %+v) = %v, want %v", tt.found, tt.state, tt.o, got, tt.want)
			}
		})
	}
}
