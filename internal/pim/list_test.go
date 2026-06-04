package pim

import (
	"bytes"
	"strings"
	"testing"
)

func sampleListRows() []ListRow {
	return []ListRow{
		{Type: "resource", Tenant: "Acme Corp", Subscription: "Acme Production", Name: "Contributor", Status: "Eligible"},
		{Type: "resource", Tenant: "Acme Corp", Subscription: "Acme Dev", Name: "Owner", Status: "Active (expires 15:42 UTC)"},
		{Type: "group", Tenant: "Acme Corp", Subscription: "—", Name: "customer-acme-admins", Status: "Eligible"},
	}
}

func TestRenderListTable_ContainsExpectedHeadersAndRows(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderListTable(&buf, sampleListRows()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"TYPE", "TENANT", "SUBSCRIPTION", "NAME", "STATUS",
		"Contributor", "customer-acme-admins", "expires 15:42 UTC"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q; got:\n%s", want, out)
		}
	}
}

func TestRenderListJSON_HasStatusField(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderListJSON(&buf, sampleListRows()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"status"`) || !strings.Contains(out, "Eligible") {
		t.Errorf("JSON missing status field; got: %s", out)
	}
}
