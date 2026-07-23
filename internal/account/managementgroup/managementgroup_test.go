package managementgroup

import "testing"

func TestNormalizeParentID(t *testing.T) {
	const prefix = "/providers/Microsoft.Management/managementGroups/"
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"contoso-root", prefix + "contoso-root"},
		{" contoso-root ", prefix + "contoso-root"},
		{prefix + "contoso-root", prefix + "contoso-root"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := normalizeParentID(tc.in); got != tc.want {
				t.Errorf("normalizeParentID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
