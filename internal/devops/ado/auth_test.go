package ado

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// fakeCred is an azcore.TokenCredential stub for exercising ResolveAuth.
type fakeCred struct {
	token azcore.AccessToken
	err   error
}

func (f *fakeCred) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return f.token, f.err
}

func TestBasicAuthHeader(t *testing.T) {
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(":abc"))
	if got := basicAuth("abc"); got != want {
		t.Errorf("basicAuth(%q) = %q, want %q", "abc", got, want)
	}
}

func TestAuthPrecedence(t *testing.T) {
	const org = "https://dev.azure.com/myorg"

	tests := []struct {
		name         string
		aad          bool
		envPATSet    bool // whether AZURE_DEVOPS_EXT_PAT is set at all (services.py:69 tests membership, not truthiness)
		envPAT       string
		orgPAT       string
		defaultPAT   string
		wantPrimary  string
		wantFallback string
		wantErr      bool
	}{
		{name: "AAD only", aad: true, wantPrimary: "aad-token"},
		{name: "AAD with env PAT as fallback", aad: true, envPATSet: true, envPAT: "envpat", wantPrimary: "aad-token", wantFallback: "envpat"},
		{name: "no AAD, env PAT wins over stored org PAT", envPATSet: true, envPAT: "envpat", orgPAT: "orgpat", wantPrimary: "envpat"},
		{name: "no AAD, stored org PAT wins over default", orgPAT: "orgpat", defaultPAT: "defpat", wantPrimary: "orgpat"},
		{name: "no AAD, default PAT used as last resort", defaultPAT: "defpat", wantPrimary: "defpat"},
		{name: "nothing available", wantErr: true},
		{name: "env PAT set but empty is used as credential, not stored PAT", envPATSet: true, orgPAT: "orgpat", wantPrimary: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
			if tt.envPATSet {
				t.Setenv("AZURE_DEVOPS_EXT_PAT", tt.envPAT)
			} else {
				old, existed := os.LookupEnv("AZURE_DEVOPS_EXT_PAT")
				os.Unsetenv("AZURE_DEVOPS_EXT_PAT")
				t.Cleanup(func() {
					if existed {
						os.Setenv("AZURE_DEVOPS_EXT_PAT", old)
					}
				})
			}

			if tt.orgPAT != "" {
				if err := SetPAT(org, tt.orgPAT); err != nil {
					t.Fatalf("SetPAT(org): %v", err)
				}
			}
			if tt.defaultPAT != "" {
				if err := SetPAT("", tt.defaultPAT); err != nil {
					t.Fatalf("SetPAT(default): %v", err)
				}
			}

			orig := getCredential
			if tt.aad {
				getCredential = func() (azcore.TokenCredential, error) {
					return &fakeCred{token: azcore.AccessToken{Token: "aad-token"}}, nil
				}
			} else {
				getCredential = func() (azcore.TokenCredential, error) {
					return nil, errors.New("az login is not present")
				}
			}
			t.Cleanup(func() { getCredential = orig })

			primary, fallback, err := ResolveAuth(context.Background(), org)

			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "aka.ms/azure-devops-cli-auth") {
					t.Fatalf("err = %v, want error containing aka.ms/azure-devops-cli-auth", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveAuth: %v", err)
			}

			if want := basicAuth(tt.wantPrimary); primary != want {
				t.Errorf("primary = %q, want %q", primary, want)
			}
			wantFallback := ""
			if tt.wantFallback != "" {
				wantFallback = basicAuth(tt.wantFallback)
			}
			if fallback != wantFallback {
				t.Errorf("fallback = %q, want %q", fallback, wantFallback)
			}
		})
	}
}

func TestAADFallsBackToPATOn401(t *testing.T) {
	testAADFallsBack(t, http.StatusUnauthorized)
}

func TestAADFallsBackToPATOn203(t *testing.T) {
	testAADFallsBack(t, http.StatusNonAuthoritativeInfo)
}

func testAADFallsBack(t *testing.T, failStatus int) {
	t.Helper()
	aad := basicAuth("aad-token")
	pat := basicAuth("pat-token")

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") == aad {
			w.WriteHeader(failStatus)
			w.Write([]byte(`{}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &Client{Org: srv.URL + "/myorg", HTTP: &http.Client{}, primary: aad, fallback: pat}
	if err := c.Do(context.Background(), Request{Path: "git/repositories", APIVersion: "5.0"}, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if requests != 2 {
		t.Errorf("got %d requests, want 2", requests)
	}
}

func TestConfigDir(t *testing.T) {
	unset := func(t *testing.T, key string) {
		t.Helper()
		old, existed := os.LookupEnv(key)
		os.Unsetenv(key)
		t.Cleanup(func() {
			if existed {
				os.Setenv(key, old)
			}
		})
	}

	t.Run("AZURE_DEVOPS_EXT_CONFIG_DIR wins", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", "/devops-dir")
		t.Setenv("AZURE_CONFIG_DIR", "/azure-dir")
		if got := configDir(); got != "/devops-dir" {
			t.Errorf("configDir() = %q, want %q", got, "/devops-dir")
		}
	})

	t.Run("falls back to AZURE_CONFIG_DIR/azuredevops", func(t *testing.T) {
		unset(t, "AZURE_DEVOPS_EXT_CONFIG_DIR")
		t.Setenv("AZURE_CONFIG_DIR", "/azure-dir")
		want := filepath.Join("/azure-dir", "azuredevops")
		if got := configDir(); got != want {
			t.Errorf("configDir() = %q, want %q", got, want)
		}
	})

	t.Run("falls back to ~/.azure/azuredevops", func(t *testing.T) {
		unset(t, "AZURE_DEVOPS_EXT_CONFIG_DIR")
		unset(t, "AZURE_CONFIG_DIR")
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".azure", "azuredevops")
		if got := configDir(); got != want {
			t.Errorf("configDir() = %q, want %q", got, want)
		}
	})
}

func TestNormalizeOrgForKey(t *testing.T) {
	tests := []struct {
		name, org, want string
	}{
		{name: "dev.azure.com keeps org segment", org: "https://dev.azure.com/MyOrg", want: "https://dev.azure.com/myorg"},
		{name: "visualstudio.com host", org: "https://myorg.visualstudio.com", want: "https://myorg.visualstudio.com"},
		{
			// _credentials.py:84 checks 'visualstudio.com' against the whole
			// url, not just the host — a dev.azure.com org whose PATH
			// happens to contain that literal string still skips the
			// org-segment append.
			name: "visualstudio.com literal in path of a dev.azure.com org",
			org:  "https://dev.azure.com/visualstudio.com",
			want: "https://dev.azure.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeOrgForKey(tt.org); got != tt.want {
				t.Errorf("normalizeOrgForKey(%q) = %q, want %q", tt.org, got, tt.want)
			}
		})
	}
}

func TestPATStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", dir)

	if err := SetPAT("https://dev.azure.com/MyOrg/", "p1"); err != nil {
		t.Fatalf("SetPAT: %v", err)
	}
	if got := GetPAT("https://dev.azure.com/myorg"); got != "p1" {
		t.Errorf("GetPAT(org, normalised case/slash) = %q, want %q", got, "p1")
	}

	if err := SetPAT("", "p2"); err != nil {
		t.Fatalf("SetPAT(default): %v", err)
	}
	if got := GetPAT("https://dev.azure.com/other"); got != "p2" {
		t.Errorf("GetPAT (default-key fallback) = %q, want %q", got, "p2")
	}

	info, err := os.Stat(filepath.Join(dir, patStoreFile))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file mode = %v, want 0600", perm)
	}
}

func TestClearPAT(t *testing.T) {
	t.Run("no credentials stored", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		if err := ClearPAT(""); err == nil || err.Error() != "No credentials were found." {
			t.Errorf("err = %v, want \"No credentials were found.\"", err)
		}
	})

	t.Run("unknown org", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		if err := SetPAT("", "p1"); err != nil {
			t.Fatalf("SetPAT: %v", err)
		}
		if err := ClearPAT("https://dev.azure.com/other"); err == nil || err.Error() != "No credentials were found." {
			t.Errorf("err = %v, want \"No credentials were found.\"", err)
		}
	})

	t.Run("org == \"\" clears every stored PAT", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", dir)
		if err := SetPAT("https://dev.azure.com/myorg", "p1"); err != nil {
			t.Fatalf("SetPAT(org): %v", err)
		}
		if err := SetPAT("", "p2"); err != nil {
			t.Fatalf("SetPAT(default): %v", err)
		}

		if err := ClearPAT(""); err != nil {
			t.Fatalf("ClearPAT(\"\"): %v", err)
		}
		if got := GetPAT("https://dev.azure.com/myorg"); got != "" {
			t.Errorf("GetPAT(org) after ClearPAT(\"\") = %q, want empty", got)
		}
		if _, err := os.Stat(filepath.Join(dir, patStoreFile)); !os.IsNotExist(err) {
			t.Errorf("PAT store file still exists after ClearPAT(\"\"): err = %v", err)
		}
	})

	t.Run("clearing one org leaves the other", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		if err := SetPAT("https://dev.azure.com/myorg", "p1"); err != nil {
			t.Fatalf("SetPAT(org): %v", err)
		}
		if err := SetPAT("", "p2"); err != nil {
			t.Fatalf("SetPAT(default): %v", err)
		}

		if err := ClearPAT("https://dev.azure.com/myorg"); err != nil {
			t.Fatalf("ClearPAT(org): %v", err)
		}
		if got := GetPAT("https://dev.azure.com/other"); got != "p2" {
			t.Errorf("GetPAT (default-key fallback) = %q, want %q", got, "p2")
		}
	})
}
