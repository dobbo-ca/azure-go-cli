package sig

import "testing"

func TestParseFeatures(t *testing.T) {
	got, err := parseFeatures("SecurityType=TrustedLaunch IsHibernateSupported=true", "V2")
	if err != nil || len(got) != 2 || *got[0].Name != "SecurityType" || *got[0].Value != "TrustedLaunch" {
		t.Fatalf("got %v, %v", got, err)
	}
	got, _ = parseFeatures("", "V2")
	if len(got) != 1 || *got[0].Value != "TrustedLaunchSupported" {
		t.Fatalf("V2 default: %v", got)
	}
	got, _ = parseFeatures("SecurityType=Standard", "V2")
	if len(got) != 0 {
		t.Fatalf("Standard should be dropped: %v", got)
	}
	got, _ = parseFeatures("", "V1")
	if len(got) != 0 {
		t.Fatalf("V1: %v", got)
	}
	if _, err = parseFeatures("bad", "V2"); err == nil {
		t.Fatal("expected usage error")
	}
}
