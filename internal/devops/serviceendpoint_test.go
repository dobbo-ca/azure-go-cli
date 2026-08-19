package devops

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// serviceendpointTestOrg is a --organization value that passes
// ado/context.go's validateOrg (it requires the literal
// "https://dev.azure.com/" prefix or a ".visualstudio.com" host — a bare
// httptest URL satisfies neither). serviceendpointTestEnv redirects its
// traffic to a local httptest.Server instead.
const serviceendpointTestOrg = "https://dev.azure.com/myorg"

// serviceendpointRedirectTransport rewrites every outbound request's
// scheme+host to target's before delegating to the real transport.
// ado.NewClient builds its own *http.Client with no Transport injection
// point, so this is what lets a validateOrg-passing organization URL
// actually reach a local httptest.Server.
type serviceendpointRedirectTransport struct {
	target *url.URL
	next   http.RoundTripper
}

func (t *serviceendpointRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	req.Host = t.target.Host
	return t.next.RoundTrip(req)
}

// serviceendpointTestEnv points the ado foundation's auth/config lookups at
// a throwaway directory and a fake PAT (config.Load() finds no profile in a
// fresh temp HOME and resolveAuth falls back to the PAT, per
// foundation-spec.md §3.2), and redirects all outbound HTTP traffic to srv.
func serviceendpointTestEnv(t *testing.T, srv *httptest.Server) {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")
	t.Setenv("HOME", t.TempDir())

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	orig := http.DefaultTransport
	http.DefaultTransport = &serviceendpointRedirectTransport{target: target, next: orig}
	t.Cleanup(func() { http.DefaultTransport = orig })
}

func TestServiceEndpointUpdateMultiCallSequence(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/myorg/MyProj/_apis/serviceendpoint/endpoints/ep1":
			w.Write([]byte(`{"id":"ep1","name":"MyEndpoint","type":"azurerm","isReady":true,"createdBy":{"displayName":"Alice"}}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/myorg/MyProj/_apis/build/authorizedresources":
			var got []map[string]any
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode PATCH body: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("PATCH body: want 1 element, got %d", len(got))
			}
			if got[0]["authorized"] != true || got[0]["id"] != "ep1" || got[0]["name"] != "MyEndpoint" || got[0]["type"] != "endpoint" {
				t.Errorf("PATCH body mismatch: %+v", got[0])
			}
			w.Write([]byte(`[]`))
		case r.Method == http.MethodGet && r.URL.Path == "/myorg/MyProj/_apis/build/authorizedresources":
			if got := r.URL.Query().Get("type"); got != "endpoint" {
				t.Errorf("GET authorizedresources type=%q, want endpoint", got)
			}
			if got := r.URL.Query().Get("id"); got != "ep1" {
				t.Errorf("GET authorizedresources id=%q, want ep1", got)
			}
			w.Write([]byte(`{"count":1,"value":[{"authorized":true}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()
	serviceendpointTestEnv(t, srv)

	cmd := serviceendpointNewUpdateCmd()
	cmd.SetArgs([]string{
		"--organization", serviceendpointTestOrg,
		"--project", "MyProj",
		"--id", "ep1",
		"--enable-for-all",
	})
	cmd.SetOut(os.Stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("want 3 calls, got %d: %v", len(calls), calls)
	}
	if calls[0] != "GET /myorg/MyProj/_apis/serviceendpoint/endpoints/ep1?api-version=5.0-preview.2" {
		t.Errorf("call 1: %s", calls[0])
	}
	if calls[1] != "PATCH /myorg/MyProj/_apis/build/authorizedresources?api-version=5.0-preview.1" {
		t.Errorf("call 2: %s", calls[1])
	}
	if calls[2] != "GET /myorg/MyProj/_apis/build/authorizedresources?api-version=5.0-preview.1&id=ep1&type=endpoint" {
		t.Errorf("call 3: %s", calls[2])
	}
}

// TestServiceEndpointUpdateEnableForAllFalse guards M5/cov-tristate: a plain
// Bool flag makes the space-separated "--enable-for-all false" bind the bare
// flag (true) and drop "false" as an unchecked positional, authorizing the
// endpoint instead of de-authorizing it (service_endpoint.py:220-222 sends
// authorized=False for this input).
func TestServiceEndpointUpdateEnableForAllFalse(t *testing.T) {
	var patchAuthorized any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/myorg/MyProj/_apis/serviceendpoint/endpoints/ep1":
			w.Write([]byte(`{"id":"ep1","name":"MyEndpoint","type":"azurerm"}`))
		case r.Method == http.MethodPatch:
			var got []map[string]any
			_ = json.NewDecoder(r.Body).Decode(&got)
			patchAuthorized = got[0]["authorized"]
			w.Write([]byte(`[]`))
		case r.Method == http.MethodGet:
			w.Write([]byte(`{"count":1,"value":[{"authorized":false}]}`))
		}
	}))
	defer srv.Close()
	serviceendpointTestEnv(t, srv)

	cmd := serviceendpointNewUpdateCmd()
	cmd.SetArgs([]string{
		"--organization", serviceendpointTestOrg,
		"--project", "MyProj",
		"--id", "ep1",
		"--enable-for-all", "false",
	})
	cmd.SetOut(os.Stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if patchAuthorized != false {
		t.Errorf("PATCH authorized = %v, want false", patchAuthorized)
	}
}

// TestServiceEndpointUpdateEnableForAllMissing guards
// cov-se-update-required-msg: Python's message when the property is entirely
// absent (service_endpoint.py:210-211), not cobra's generic "required flag(s)".
func TestServiceEndpointUpdateEnableForAllMissing(t *testing.T) {
	cmd := serviceendpointNewUpdateCmd()
	cmd.SetArgs([]string{
		"--organization", serviceendpointTestOrg,
		"--id", "ep1",
	})
	err := cmd.Execute()
	want := "Atleast one property to be updated must be specified."
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestServiceEndpointDeleteDeepParam(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantDeep string
	}{
		{name: "default false", args: nil, wantDeep: "false"},
		{name: "explicit true", args: []string{"--deep"}, wantDeep: "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotDeep string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotDeep = r.URL.Query().Get("deep")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()
			serviceendpointTestEnv(t, srv)

			cmd := serviceendpointNewDeleteCmd()
			args := append([]string{
				"--organization", serviceendpointTestOrg,
				"--project", "MyProj",
				"--id", "ep1",
				"--yes",
			}, tt.args...)
			cmd.SetArgs(args)
			cmd.SetOut(os.Stdout)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute: %v", err)
			}

			if gotMethod != http.MethodDelete {
				t.Errorf("method = %s, want DELETE", gotMethod)
			}
			if gotDeep != tt.wantDeep {
				t.Errorf("deep = %s, want %s", gotDeep, tt.wantDeep)
			}
		})
	}
}

func TestServiceEndpointCreatePassthrough(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"name":"conn1","type":"generic","url":"https://example.com","authorization":{"scheme":"UsernamePassword","parameters":{"username":"u","password":"p"}}}`), 0600); err != nil {
		t.Fatal(err)
	}

	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"new1","name":"conn1"}`))
	}))
	defer srv.Close()
	serviceendpointTestEnv(t, srv)

	cmd := serviceendpointNewCreateCmd()
	cmd.SetArgs([]string{
		"--organization", serviceendpointTestOrg,
		"--project", "MyProj",
		"--service-endpoint-configuration", cfgPath,
	})
	cmd.SetOut(os.Stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/myorg/MyProj/_apis/serviceendpoint/endpoints" {
		t.Errorf("path = %s", gotPath)
	}
	if gotBody["name"] != "conn1" || gotBody["type"] != "generic" {
		t.Errorf("body passthrough mismatch: %+v", gotBody)
	}
}

func TestServiceEndpointAzureRMCreateBody(t *testing.T) {
	t.Run("spnKey via env var", func(t *testing.T) {
		var gotBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"ep2"}`))
		}))
		defer srv.Close()
		serviceendpointTestEnv(t, srv)
		t.Setenv("AZURE_DEVOPS_EXT_AZURE_RM_SERVICE_PRINCIPAL_KEY", "supersecret")

		cmd := serviceendpointNewAzureRMCreateCmd()
		cmd.SetArgs([]string{
			"--organization", serviceendpointTestOrg,
			"--project", "MyProj",
			"--name", "MyConn",
			"--azure-rm-tenant-id", "tenant1",
			"--azure-rm-service-principal-id", "sp1",
			"--azure-rm-subscription-id", "sub1",
			"--azure-rm-subscription-name", "SubName",
		})
		cmd.SetOut(os.Stdout)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}

		if gotBody["type"] != "azurerm" || gotBody["url"] != "https://management.azure.com/" {
			t.Fatalf("body mismatch: %+v", gotBody)
		}
		auth, _ := gotBody["authorization"].(map[string]any)
		params, _ := auth["parameters"].(map[string]any)
		if params["authenticationType"] != "spnKey" || params["serviceprincipalkey"] != "supersecret" {
			t.Errorf("authorization.parameters mismatch: %+v", params)
		}
		data, _ := gotBody["data"].(map[string]any)
		if data["subscriptionId"] != "sub1" || data["environment"] != "AzureCloud" || data["creationMode"] != "Manual" {
			t.Errorf("data mismatch: %+v", data)
		}
	})

	t.Run("spnCertificate via file", func(t *testing.T) {
		pemPath := filepath.Join(t.TempDir(), "cert.pem")
		if err := os.WriteFile(pemPath, []byte("-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----\n"), 0600); err != nil {
			t.Fatal(err)
		}

		var gotBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"ep3"}`))
		}))
		defer srv.Close()
		serviceendpointTestEnv(t, srv)

		cmd := serviceendpointNewAzureRMCreateCmd()
		cmd.SetArgs([]string{
			"--organization", serviceendpointTestOrg,
			"--project", "MyProj",
			"--name", "MyConn",
			"--azure-rm-tenant-id", "tenant1",
			"--azure-rm-service-principal-id", "sp1",
			"--azure-rm-subscription-id", "sub1",
			"--azure-rm-subscription-name", "SubName",
			"--azure-rm-service-principal-certificate-path", pemPath,
		})
		cmd.SetOut(os.Stdout)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}

		auth, _ := gotBody["authorization"].(map[string]any)
		params, _ := auth["parameters"].(map[string]any)
		if params["authenticationType"] != "spnCertificate" {
			t.Errorf("authenticationType = %v, want spnCertificate", params["authenticationType"])
		}
		if got, _ := params["servicePrincipalCertificate"].(string); got == "" {
			t.Errorf("servicePrincipalCertificate not set")
		}
	})
}

func TestServiceEndpointGitHubCreateBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"ep4"}`))
	}))
	defer srv.Close()
	serviceendpointTestEnv(t, srv)
	t.Setenv("AZURE_DEVOPS_EXT_GITHUB_PAT", "ghp_test")

	cmd := serviceendpointNewGitHubCreateCmd()
	cmd.SetArgs([]string{
		"--organization", serviceendpointTestOrg,
		"--project", "MyProj",
		"--name", "MyGitHub",
		"--github-url", "https://github.com/example/repo",
	})
	cmd.SetOut(os.Stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotBody["type"] != "github" || gotBody["url"] != "https://github.com/example/repo" {
		t.Fatalf("body mismatch: %+v", gotBody)
	}
	auth, _ := gotBody["authorization"].(map[string]any)
	if auth["scheme"] != "PersonalAccessToken" {
		t.Errorf("scheme = %v", auth["scheme"])
	}
	params, _ := auth["parameters"].(map[string]any)
	if params["accessToken"] != "ghp_test" {
		t.Errorf("accessToken = %v", params["accessToken"])
	}
}

// TestServiceEndpointSecretFromEnvOrPrompt_EmptyEnvValueIsUsedVerbatim guards
// m5/X-10: service_endpoint.py:123,165 test `in os.environ`, so an
// exported-but-empty var is used as-is rather than falling through to the
// TTY prompt/error.
func TestServiceEndpointSecretFromEnvOrPrompt_EmptyEnvValueIsUsedVerbatim(t *testing.T) {
	t.Setenv("AZURE_DEVOPS_EXT_TEST_SECRET", "")

	got, err := serviceendpointSecretFromEnvOrPrompt("AZURE_DEVOPS_EXT_TEST_SECRET", "label", "non-tty error")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != "" {
		t.Errorf("got = %q, want empty string", got)
	}
}
