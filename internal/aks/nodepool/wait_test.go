package nodepool

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
		{name: "deleted and gone", found: false, state: "", deleted: true, exists: false, want: true},
		{name: "exists and present", found: true, state: "Deleting", deleted: false, exists: true, want: true},
		{name: "default succeeded", found: true, state: "Succeeded", deleted: false, exists: false, want: true},
		{name: "default in progress", found: true, state: "InProgress", deleted: false, exists: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := waitDone(tt.found, tt.state, tt.deleted, tt.exists); got != tt.want {
				t.Errorf("waitDone(%v, %q, %v, %v) = %v, want %v", tt.found, tt.state, tt.deleted, tt.exists, got, tt.want)
			}
		})
	}
}
