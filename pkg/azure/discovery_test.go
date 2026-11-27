package azure

import (
	"errors"
	"fmt"
	"testing"
)

// The exact error surfaced when a tenant's cached refresh token has expired due
// to Conditional Access sign-in frequency (AADSTS70043). It must be treated as
// "needs interactive re-auth", same as an MFA challenge.
const reportedTokenExpiredErr = `failed to acquire token silently: http call(https://login.microsoftonline.com/729c6ac2-4fef-4db3-bae0-25ccb8bd9902/oauth2/v2.0/token)(POST) error: reply status code was 400:
{"error":"invalid_grant","error_description":"AADSTS70043: The refresh token has expired or is invalid due to sign-in frequency checks by conditional access. The token was issued on 2026-07-22T05:15:31.6070000Z and the maximum allowed lifetime for this request is 36000. Trace ID: 1e1276d7-8b9a-4d25-b93b-83b384a30000 Correlation ID: a7bfd647-1f22-41ed-bafa-8ff789a51092 Timestamp: 2026-07-23 04:41:54Z","error_codes":[70043],"timestamp":"2026-07-23 04:41:54Z","trace_id":"1e1276d7-8b9a-4d25-b93b-83b384a30000","correlation_id":"a7bfd647-1f22-41ed-bafa-8ff789a51092","suberror":"token_expired"}`

func TestNeedsInteractiveAuth(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"MFA required (AADSTS50076)", errors.New("failed to list: AADSTS50076: due to a configuration change made by your administrator"), true},
		{"refresh token expired (AADSTS70043)", errors.New(reportedTokenExpiredErr), true},
		{"refresh token revoked (AADSTS50173)", errors.New("reply status code was 400:\n" + revokedGrantBody), true},
		{"refresh token inactive (AADSTS700082)", errors.New(`reply status code was 400:\n{"error":"invalid_grant","error_description":"AADSTS700082: The refresh token has expired due to inactivity.","error_codes":[700082]}`), true},
		{"MFA enrollment (AADSTS50079)", errors.New(`reply status code was 400:\n{"error":"interaction_required","error_description":"AADSTS50079: Due to a configuration change made by your administrator, you must enroll in multi-factor authentication.","error_codes":[50079]}`), true},
		{"disabled account (AADSTS50057)", errors.New(`reply status code was 400:\n{"error":"invalid_grant","error_description":"AADSTS50057: The user account is disabled.","error_codes":[50057]}`), false},
		{"wrapped LoginRequiredError", fmt.Errorf("list: %w", &LoginRequiredError{TenantID: "t", Err: errors.New("x")}), true},
		{"transient AAD error", errors.New(`reply status code was 503:\n{"error":"temporarily_unavailable","error_description":"AADSTS90033: A transient error has occurred."}`), false},
		{"unrelated permission error", errors.New("AADSTS500011: The resource principal was not found in the tenant"), false},
		{"plain network error", errors.New("dial tcp: connection refused"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsInteractiveAuth(tc.err); got != tc.want {
				t.Errorf("needsInteractiveAuth() = %v, want %v", got, tc.want)
			}
		})
	}
}
