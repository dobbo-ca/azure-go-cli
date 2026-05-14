package credplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

func TestDetermineAPIVersion(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		want    string
		wantErr bool
	}{
		{name: "empty env defaults to v1beta1", env: "", want: APIVersionV1Beta1},
		{name: "explicit v1beta1", env: `{"apiVersion":"client.authentication.k8s.io/v1beta1","kind":"ExecCredential"}`, want: APIVersionV1Beta1},
		{name: "explicit v1", env: `{"apiVersion":"client.authentication.k8s.io/v1","kind":"ExecCredential"}`, want: APIVersionV1},
		{name: "envelope without apiVersion defaults to v1beta1", env: `{"kind":"ExecCredential"}`, want: APIVersionV1Beta1},
		{name: "unknown apiVersion errors", env: `{"apiVersion":"bogus/v9"}`, wantErr: true},
		{name: "malformed json errors", env: `{not json`, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetermineAPIVersion(tc.env)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (result=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderExecCredential(t *testing.T) {
	expiry := time.Date(2026, 5, 14, 15, 30, 0, 0, time.UTC)
	token := azcore.AccessToken{Token: "abc.def.ghi", ExpiresOn: expiry}

	cases := []struct {
		name       string
		apiVersion string
	}{
		{"v1beta1", APIVersionV1Beta1},
		{"v1", APIVersionV1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := RenderExecCredential(token, tc.apiVersion, &buf); err != nil {
				t.Fatalf("RenderExecCredential: %v", err)
			}
			var got ExecCredential
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("output not valid JSON: %v\noutput=%s", err, buf.String())
			}
			if got.Kind != "ExecCredential" {
				t.Errorf("kind=%q, want ExecCredential", got.Kind)
			}
			if got.APIVersion != tc.apiVersion {
				t.Errorf("apiVersion=%q, want %q", got.APIVersion, tc.apiVersion)
			}
			if got.Status.Token != "abc.def.ghi" {
				t.Errorf("token=%q, want abc.def.ghi", got.Status.Token)
			}
			if !got.Status.ExpirationTimestamp.Equal(expiry) {
				t.Errorf("expirationTimestamp=%v, want %v", got.Status.ExpirationTimestamp, expiry)
			}
		})
	}
}

// fakeCred is an azcore.TokenCredential we can rig to return any access token
// or error from GetToken — enough to exercise the GetToken composition logic.
type fakeCred struct {
	token     azcore.AccessToken
	err       error
	gotScopes []string
}

func (f *fakeCred) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	f.gotScopes = opts.Scopes
	return f.token, f.err
}

func TestGetToken_HappyPath(t *testing.T) {
	expiry := time.Date(2026, 5, 14, 15, 30, 0, 0, time.UTC)
	cred := &fakeCred{token: azcore.AccessToken{Token: "tok", ExpiresOn: expiry}}

	var buf bytes.Buffer
	opts := GetTokenOptions{
		ServerID:          "server-id-x",
		CredentialFactory: func() (azcore.TokenCredential, error) { return cred, nil },
		Stdout:            &buf,
		ExecInfoEnv:       `{"apiVersion":"client.authentication.k8s.io/v1"}`,
	}
	if err := GetToken(context.Background(), opts); err != nil {
		t.Fatalf("GetToken: %v", err)
	}

	if len(cred.gotScopes) != 1 || cred.gotScopes[0] != "server-id-x/.default" {
		t.Errorf("scopes=%v, want [server-id-x/.default]", cred.gotScopes)
	}
	if !strings.Contains(buf.String(), `"apiVersion":"client.authentication.k8s.io/v1"`) {
		t.Errorf("output missing v1 apiVersion: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"token":"tok"`) {
		t.Errorf("output missing token: %s", buf.String())
	}
}

func TestGetToken_RequiresServerID(t *testing.T) {
	err := GetToken(context.Background(), GetTokenOptions{
		CredentialFactory: func() (azcore.TokenCredential, error) { return &fakeCred{}, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "server-id") {
		t.Fatalf("want server-id required error, got %v", err)
	}
}

func TestGetToken_CredentialFactoryError(t *testing.T) {
	err := GetToken(context.Background(), GetTokenOptions{
		ServerID:          "x",
		CredentialFactory: func() (azcore.TokenCredential, error) { return nil, fmt.Errorf("boom") },
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want credential error surfaced, got %v", err)
	}
}

func TestGetToken_MintError(t *testing.T) {
	cred := &fakeCred{err: fmt.Errorf("mint failed")}
	err := GetToken(context.Background(), GetTokenOptions{
		ServerID:          "x",
		CredentialFactory: func() (azcore.TokenCredential, error) { return cred, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "mint failed") {
		t.Fatalf("want mint error surfaced, got %v", err)
	}
}
