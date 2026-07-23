package lock

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
)

func TestParseLockLevel(t *testing.T) {
	cases := []struct {
		in      string
		want    armlocks.LockLevel
		wantErr bool
	}{
		{"CanNotDelete", armlocks.LockLevelCanNotDelete, false},
		{"cannotdelete", armlocks.LockLevelCanNotDelete, false},
		{"ReadOnly", armlocks.LockLevelReadOnly, false},
		{"readonly", armlocks.LockLevelReadOnly, false},
		{" ReadOnly ", armlocks.LockLevelReadOnly, false},
		{"", "", true},
		{"Bogus", "", true},
		{"NotSpecified", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseLockLevel(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseLockLevel(%q) = %v, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseLockLevel(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("parseLockLevel(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
