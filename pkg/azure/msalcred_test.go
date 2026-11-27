package azure

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
)

const (
	testTenant = "729c6ac2-4fef-4db3-bae0-25ccb8bd9902"
	testHomeID = "00000000-0000-0000-0000-0000000000aa.d4cc8940-0000-0000-0000-000000000000"
)

// revokedGrantBody is the token endpoint reply after a password reset
// revoked the cached refresh token.
const revokedGrantBody = `{"error":"invalid_grant","error_description":"AADSTS50173: The provided grant has expired due to it being revoked, a fresh auth token is needed. The user might have changed or reset their password.","error_codes":[50173]}`

// memCache holds an MSAL cache with one account and its refresh token, and
// no access token, so AcquireTokenSilent must redeem the refresh token.
type memCache struct{ data []byte }

func (m *memCache) Replace(_ context.Context, u cache.Unmarshaler, _ cache.ReplaceHints) error {
	return u.Unmarshal(m.data)
}

func (m *memCache) Export(_ context.Context, c cache.Marshaler, _ cache.ExportHints) error {
	var err error
	m.data, err = c.Marshal()
	return err
}

// fakeAAD answers tenant discovery and fails every token request with body.
type fakeAAD struct{ body string }

func (f fakeAAD) Do(req *http.Request) (*http.Response, error) {
	status, body := http.StatusOK, `{"authorization_endpoint":"https://login.microsoftonline.com/`+testTenant+`/oauth2/v2.0/authorize","token_endpoint":"https://login.microsoftonline.com/`+testTenant+`/oauth2/v2.0/token","issuer":"https://login.microsoftonline.com/`+testTenant+`/v2.0"}`
	if strings.HasSuffix(req.URL.Path, "/oauth2/v2.0/token") {
		status, body = http.StatusBadRequest, f.body
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func (fakeAAD) CloseIdleConnections() {}

func TestMSALSilentCredential_RevokedRefreshTokenAsksForLogin(t *testing.T) {
	seed := `{
		"Account": {"a": {"home_account_id": "` + testHomeID + `", "environment": "login.microsoftonline.com", "realm": "organizations", "local_account_id": "00000000-0000-0000-0000-0000000000aa", "username": "user@example.com", "authority_type": "MSSTS"}},
		"RefreshToken": {"r": {"home_account_id": "` + testHomeID + `", "environment": "login.microsoftonline.com", "credential_type": "RefreshToken", "client_id": "04b07795-8ddb-461a-bbee-02f9e1bf7b46", "secret": "revoked-rt", "family_id": "1"}},
		"AppMetadata": {"m": {"environment": "login.microsoftonline.com", "client_id": "04b07795-8ddb-461a-bbee-02f9e1bf7b46", "family_id": "1"}}
	}`
	record := azidentity.AuthenticationRecord{
		Authority:     "login.microsoftonline.com",
		HomeAccountID: testHomeID,
		TenantID:      "organizations",
		Username:      "user@example.com",
	}
	cred, err := newMSALSilentCredential(testTenant, record,
		public.WithCache(&memCache{data: []byte(seed)}),
		public.WithHTTPClient(fakeAAD{body: revokedGrantBody}),
		public.WithInstanceDiscovery(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cred.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{"https://management.azure.com/.default"}})
	if err == nil {
		t.Fatal("GetToken succeeded with a revoked refresh token")
	}
	var loginErr *LoginRequiredError
	if !errors.As(err, &loginErr) {
		t.Fatalf("want *LoginRequiredError, got %T: %v", err, err)
	}
	msg := err.Error()
	for _, want := range []string{"AADSTS50173", "Please run 'az login'", "az login --tenant " + testTenant} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not contain %q", msg, want)
		}
	}
	if strings.Contains(msg, "http call(") {
		t.Errorf("error leaks the raw MSAL HTTP text: %q", msg)
	}
	if !NeedsInteractiveAuth(err) {
		t.Error("NeedsInteractiveAuth = false for a revoked refresh token")
	}
}
