package credplugin

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
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
