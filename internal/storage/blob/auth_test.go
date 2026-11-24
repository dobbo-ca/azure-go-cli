package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

var authEnv = []string{
	"AZURE_STORAGE_AUTH_MODE", "AZURE_STORAGE_ACCOUNT", "AZURE_STORAGE_KEY",
	"AZURE_STORAGE_SAS_TOKEN", "AZURE_STORAGE_CONNECTION_STRING", "AZURE_STORAGE_SERVICE_ENDPOINT",
}

func clearAuthEnv(t *testing.T) {
	for _, k := range authEnv {
		t.Setenv(k, "")
	}
}

// fakeFetch records lookups and returns a fixed key or error.
type fakeFetch struct {
	key   string
	err   error
	calls []string
}

func (f *fakeFetch) fetch(_ context.Context, name string) (string, error) {
	f.calls = append(f.calls, name)
	return f.key, f.err
}

func TestResolveAuthDefaultQueriesAccountKey(t *testing.T) {
	clearAuthEnv(t)
	f := &fakeFetch{key: "k1"}
	var warn bytes.Buffer

	r, err := resolveAuth(context.Background(), authInputs{accountName: "acct"}, f.fetch, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if r.login || r.accountKey != "k1" || r.accountName != "acct" {
		t.Errorf("got %+v, want key auth with fetched key", r)
	}
	if len(f.calls) != 1 || f.calls[0] != "acct" {
		t.Errorf("fetch calls = %v, want [acct]", f.calls)
	}
	if !strings.Contains(warn.String(), "we will query for account key") {
		t.Errorf("missing no-credentials warning:\n%s", warn.String())
	}
}

func TestResolveAuthKeyLookupFailureIsAnonymous(t *testing.T) {
	clearAuthEnv(t)
	f := &fakeFetch{err: errors.New("no listKeys permission")}
	var warn bytes.Buffer

	r, err := resolveAuth(context.Background(), authInputs{accountName: "acct", authMode: "key"}, f.fetch, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if r.login || r.accountKey != "" || r.sasToken != "" {
		t.Errorf("got %+v, want anonymous", r)
	}
	if !strings.Contains(warn.String(), "Skip querying account key due to failure: no listKeys permission") {
		t.Errorf("missing failure warning:\n%s", warn.String())
	}
}

func TestResolveAuthLogin(t *testing.T) {
	clearAuthEnv(t)
	f := &fakeFetch{}
	var warn bytes.Buffer

	r, err := resolveAuth(context.Background(), authInputs{accountName: "acct", authMode: "login", accountKey: "k", sasToken: "s"}, f.fetch, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if !r.login || len(f.calls) != 0 {
		t.Errorf("got %+v with %d fetches, want login and no fetch", r, len(f.calls))
	}
	if !strings.Contains(warn.String(), `the following arguments are ignored: --account-key ,--sas-token`) {
		t.Errorf("missing ignored-args warning:\n%s", warn.String())
	}
}

func TestResolveAuthModeCaseInsensitive(t *testing.T) {
	clearAuthEnv(t)
	r, err := resolveAuth(context.Background(), authInputs{accountName: "acct", authMode: "Login"}, (&fakeFetch{}).fetch, &bytes.Buffer{})
	if err != nil || !r.login {
		t.Errorf("got %+v, %v; want login", r, err)
	}
}

func TestResolveAuthLoginFromEnv(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("AZURE_STORAGE_AUTH_MODE", "login")
	t.Setenv("AZURE_STORAGE_ACCOUNT", "envacct")

	r, err := resolveAuth(context.Background(), authInputs{}, (&fakeFetch{}).fetch, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.login || r.accountName != "envacct" {
		t.Errorf("got %+v, want login for envacct", r)
	}
}

func TestResolveAuthPrecedence(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		in   authInputs
		want resolvedAuth
	}{
		{
			name: "explicit key, no lookup",
			in:   authInputs{accountName: "acct", accountKey: "k"},
			want: resolvedAuth{accountName: "acct", accountKey: "k"},
		},
		{
			name: "sas token strips leading ?",
			in:   authInputs{accountName: "acct", sasToken: "?sv=1&sig=x"},
			want: resolvedAuth{accountName: "acct", sasToken: "sv=1&sig=x"},
		},
		{
			name: "connection string from env",
			env:  map[string]string{"AZURE_STORAGE_CONNECTION_STRING": "AccountName=a;AccountKey=b"},
			in:   authInputs{},
			want: resolvedAuth{connectionString: "AccountName=a;AccountKey=b"},
		},
		{
			name: "explicit key beats env connection string",
			env:  map[string]string{"AZURE_STORAGE_CONNECTION_STRING": "AccountName=a;AccountKey=b"},
			in:   authInputs{accountName: "acct", accountKey: "k"},
			want: resolvedAuth{accountName: "acct", accountKey: "k"},
		},
		{
			name: "account and key from env",
			env:  map[string]string{"AZURE_STORAGE_ACCOUNT": "envacct", "AZURE_STORAGE_KEY": "envkey"},
			in:   authInputs{},
			want: resolvedAuth{accountName: "envacct", accountKey: "envkey"},
		},
		{
			name: "env key ignored when sas given",
			env:  map[string]string{"AZURE_STORAGE_KEY": "envkey"},
			in:   authInputs{accountName: "acct", sasToken: "sig=x"},
			want: resolvedAuth{accountName: "acct", sasToken: "sig=x"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearAuthEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			f := &fakeFetch{}
			got, err := resolveAuth(context.Background(), tc.in, f.fetch, &bytes.Buffer{})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
			if len(f.calls) != 0 {
				t.Errorf("unexpected key lookup: %v", f.calls)
			}
		})
	}
}

func TestResolveAuthErrors(t *testing.T) {
	cases := map[string]authInputs{
		"bad auth mode":     {accountName: "acct", authMode: "token"},
		"no account":        {},
		"no account login":  {authMode: "login"},
		"bad account name":  {accountName: "https://evil/?sig=secret"},
		"endpoint no creds": {blobEndpoint: "https://acct.blob.core.windows.net"},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			clearAuthEnv(t)
			if _, err := resolveAuth(context.Background(), in, (&fakeFetch{}).fetch, &bytes.Buffer{}); err == nil {
				t.Error("want error, got nil")
			}
		})
	}
}

func TestStorageErrorHints(t *testing.T) {
	respErr := func(status int, code string) error {
		return fmt.Errorf("wrapped: %w", &azcore.ResponseError{StatusCode: status, ErrorCode: code})
	}
	cases := []struct {
		err  error
		want string
	}{
		{respErr(403, "AuthorizationPermissionMismatch"), `"--auth-mode" parameter and "key" value`},
		{respErr(403, "AuthorizationFailure"), "network rules"},
		{respErr(403, "AuthenticationFailed"), "invalid account key"},
	}
	for _, tc := range cases {
		got := storageError(tc.err)
		if !strings.Contains(got.Error(), tc.want) {
			t.Errorf("storageError(%v) = %q, want it to contain %q", tc.err, got, tc.want)
		}
		var re *azcore.ResponseError
		if !errors.As(got, &re) {
			t.Errorf("storageError dropped the ResponseError")
		}
	}

	for _, err := range []error{respErr(404, "BlobNotFound"), respErr(403, "Other"), errors.New("plain")} {
		if got := storageError(err); got != err {
			t.Errorf("storageError(%v) changed an error it has no hint for: %v", err, got)
		}
	}
}
