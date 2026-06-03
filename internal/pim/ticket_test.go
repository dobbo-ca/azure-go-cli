package pim

import "testing"

func TestParseTicket(t *testing.T) {
	cases := []struct {
		in         string
		wantSystem string
		wantNumber string
	}{
		{"Jira:TEC-1234", "Jira", "TEC-1234"},
		{"ServiceNow:INC0001", "ServiceNow", "INC0001"},
		{"TEC-1234", "", "TEC-1234"},
		{"", "", ""},
		{":INC-1", "", "INC-1"},
		{"Jira:TEC:1234", "Jira", "TEC:1234"}, // only first colon splits
	}
	for _, c := range cases {
		gotSys, gotNum := ParseTicket(c.in)
		if gotSys != c.wantSystem || gotNum != c.wantNumber {
			t.Errorf("ParseTicket(%q) = (%q,%q); want (%q,%q)",
				c.in, gotSys, gotNum, c.wantSystem, c.wantNumber)
		}
	}
}
