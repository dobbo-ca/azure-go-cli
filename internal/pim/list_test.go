package pim

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	pimvendor "github.com/cdobbyn/azure-go-cli/internal/pim/vendor"
)

func sampleListRows() []ListRow {
	return []ListRow{
		{Type: "resource", Tenant: "Acme Corp", Subscription: "Acme Production", Name: "Contributor", Status: "Eligible"},
		{Type: "resource", Tenant: "Acme Corp", Subscription: "Acme Dev", Name: "Owner", Status: "Active (expires 15:42 UTC)"},
	}
}

func TestRenderListTable_ContainsExpectedHeadersAndRows(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderListTable(&buf, sampleListRows()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"TYPE", "TENANT", "SUBSCRIPTION", "NAME", "STATUS",
		"Contributor", "Owner", "expires 15:42 UTC"} {
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

// pimAPI answers every PIM request with an empty list and records the hosts.
type pimAPI struct{ hosts []string }

func (p *pimAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	p.hosts = append(p.hosts, req.URL.Host)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"value":[]}`)),
		Request:    req,
	}, nil
}

// Entra group PIM was removed: its API (api.azrbac.mspim.azure.com) stopped
// accepting Azure CLI tokens, which surfaced as "401 InvalidTenantId". list
// must only call ARM and ask for one ARM token.
func TestCollectRows_OnlyCallsARM(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	api := &pimAPI{}
	orig := http.DefaultClient.Transport
	http.DefaultClient.Transport = api
	t.Cleanup(func() { http.DefaultClient.Transport = orig })

	fc := &fakeCred{token: "arm-token"}
	if _, err := collectRows(NewTokenSource(fc), pimvendor.AzureClient{ARMBaseURL: pimvendor.ARM_GLOBAL_BASE_URL}); err != nil {
		t.Fatal(err)
	}
	if len(fc.calls) != 1 || strings.Join(fc.calls[0].Scopes, " ") != "https://management.azure.com/.default" {
		t.Errorf("token requests = %+v, want one ARM scope", fc.calls)
	}
	if strings.Join(api.hosts, " ") != "management.azure.com" {
		t.Errorf("requests went to %v, want only management.azure.com", api.hosts)
	}
}

func TestListRejectsGroupType(t *testing.T) {
	cmd := newListCmd()
	cmd.SetArgs([]string{"--type", "group"})
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.Flags().String("output", "", "")
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "Entra group PIM was removed") {
		t.Fatalf("err = %v, want the group-removed error", err)
	}
}
