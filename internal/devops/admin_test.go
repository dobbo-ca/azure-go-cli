package devops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// adminTestClient builds an *ado.Client against srv with a hermetic,
// network-free auth path (a fake PAT via env var; AZ_SESSION points the
// foundation's config.Load() at a profile file that can't exist, so the AAD
// attempt fails fast with no network call).
func adminTestClient(t *testing.T, srv *httptest.Server) *ado.Client {
	t.Helper()
	t.Setenv("AZ_SESSION", "admin-banner-test-"+t.Name())
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	client, err := ado.NewClient(context.Background(), srv.URL+"/testorg")
	if err != nil {
		t.Fatalf("ado.NewClient: %v", err)
	}
	return client
}

func TestAdminBannerAddDo(t *testing.T) {
	var gotMethod, gotURL string
	var gotBody map[string]map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotURL = r.URL.String()
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	client := adminTestClient(t, srv)
	cmd := &cobra.Command{}

	expiration := "2019-06-10T17:21:00Z"
	if err := adminBannerAddDo(context.Background(), cmd, client, "hello", "warning", "my-id", &expiration); err != nil {
		t.Fatalf("adminBannerAddDo: %v", err)
	}

	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	wantURL := "/testorg/_apis/Settings/Entries/host?api-version=5.0-preview.1"
	if gotURL != wantURL {
		t.Errorf("url = %q, want %q", gotURL, wantURL)
	}

	entry, ok := gotBody["GlobalMessageBanners/my-id"]
	if !ok {
		t.Fatalf("body missing key GlobalMessageBanners/my-id: %v", gotBody)
	}
	if entry["message"] != "hello" {
		t.Errorf("message = %v, want hello", entry["message"])
	}
	if entry["level"] != "warning" {
		t.Errorf("level = %v, want warning", entry["level"])
	}
	if entry["expirationDate"] != "2019-06-10T17:21:00+00:00" {
		t.Errorf("expirationDate = %v, want 2019-06-10T17:21:00+00:00", entry["expirationDate"])
	}
}

func TestAdminBannerAddDo_InvalidType(t *testing.T) {
	client := adminTestClient(t, httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request %s %s", r.Method, r.URL)
	})))
	err := adminBannerAddDo(context.Background(), &cobra.Command{}, client, "hello", "bogus", "my-id", nil)
	if err == nil {
		t.Fatal("expected an error for an invalid --type")
	}
}

func TestAdminBannerUpdateDo(t *testing.T) {
	tests := []struct {
		name           string
		message        *string
		bannerType     *string
		expiration     *string
		wantMessage    any
		wantLevel      any
		wantExpiration any
	}{
		{
			// Only --message supplied: --type and --expiration are omitted,
			// so their values carry over from the existing entry
			// (banner.py:96-114, the "keep existing" branch).
			name:           "message only keeps existing level and expiration",
			message:        adminStrPtr("new message"),
			wantMessage:    "new message",
			wantLevel:      "info",
			wantExpiration: "2020-01-01T00:00:00+00:00",
		},
		{
			// An explicit empty --expiration clears it, independently of
			// message/type being omitted (banner.py:84-87, 111-114) — the
			// 3-way state machine this port must not collapse to a 2-way
			// "provided vs not" check.
			name:           "empty expiration clears it, existing message/level kept",
			expiration:     adminStrPtr(""),
			wantMessage:    "old message",
			wantLevel:      "info",
			wantExpiration: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			var patchBody map[string]map[string]any

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method+" "+r.URL.String())
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodGet:
					w.Write([]byte(`{"count":1,"value":{"existing-id":{"message":"old message","level":"info","expirationDate":"2020-01-01T00:00:00+00:00"}}}`))
				case http.MethodPatch:
					b, _ := io.ReadAll(r.Body)
					_ = json.Unmarshal(b, &patchBody)
					w.Write([]byte("{}"))
				}
			}))
			defer srv.Close()

			client := adminTestClient(t, srv)
			cmd := &cobra.Command{}

			if err := adminBannerUpdateDo(context.Background(), cmd, client, "existing-id", tt.message, tt.bannerType, tt.expiration); err != nil {
				t.Fatalf("adminBannerUpdateDo: %v", err)
			}

			wantCalls := []string{
				"GET /testorg/_apis/Settings/Entries/host/GlobalMessageBanners?api-version=5.0-preview.1",
				"PATCH /testorg/_apis/Settings/Entries/host?api-version=5.0-preview.1",
			}
			if fmt.Sprint(calls) != fmt.Sprint(wantCalls) {
				t.Fatalf("calls = %v, want %v", calls, wantCalls)
			}

			entry, ok := patchBody["GlobalMessageBanners/existing-id"]
			if !ok {
				t.Fatalf("PATCH body missing key GlobalMessageBanners/existing-id: %v", patchBody)
			}
			if entry["message"] != tt.wantMessage {
				t.Errorf("message = %v, want %v", entry["message"], tt.wantMessage)
			}
			if entry["level"] != tt.wantLevel {
				t.Errorf("level = %v, want %v", entry["level"], tt.wantLevel)
			}
			if entry["expirationDate"] != tt.wantExpiration {
				t.Errorf("expirationDate = %v, want %v", entry["expirationDate"], tt.wantExpiration)
			}
		})
	}
}

func TestAdminBannerUpdateDo_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":0,"value":{}}`))
	}))
	defer srv.Close()

	client := adminTestClient(t, srv)
	err := adminBannerUpdateDo(context.Background(), &cobra.Command{}, client, "missing-id", adminStrPtr("hi"), nil, nil)
	if err == nil || err.Error() != "The following banner was not found: missing-id" {
		t.Fatalf("err = %v, want not-found message", err)
	}
}

func TestAdminBannerRemoveDo(t *testing.T) {
	var gotMethod, gotURL string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := adminTestClient(t, srv)
	if err := adminBannerRemoveDo(context.Background(), client, "banner-id-1"); err != nil {
		t.Fatalf("adminBannerRemoveDo: %v", err)
	}

	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	// %2F must survive verbatim: the DELETE route embeds the full setting
	// key ("GlobalMessageBanners/banner-id-1") as a single path segment
	// (settings_client.py's route_values are not further split on "/").
	wantURL := "/testorg/_apis/Settings/Entries/host/GlobalMessageBanners%2Fbanner-id-1?api-version=5.0-preview.1"
	if gotURL != wantURL {
		t.Errorf("url = %q, want %q", gotURL, wantURL)
	}
}

func TestAdminBannerShowDo(t *testing.T) {
	var callCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":1,"value":{"banner-1":{"message":"hi","level":"info"}}}`))
	}))
	defer srv.Close()

	client := adminTestClient(t, srv)
	cmd := &cobra.Command{}

	if err := adminBannerShowDo(context.Background(), cmd, client, "banner-1"); err != nil {
		t.Fatalf("adminBannerShowDo: %v", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (show does a single list-all, no separate get-by-id call)", callCount)
	}

	err := adminBannerShowDo(context.Background(), cmd, client, "missing")
	if err == nil || err.Error() != "The following banner was not found: missing" {
		t.Fatalf("err = %v, want not-found message", err)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 after a second show call", callCount)
	}
}

func adminStrPtr(s string) *string { return &s }
