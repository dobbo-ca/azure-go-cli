package support

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/require"
)

func env(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }

func TestSupportLanguage(t *testing.T) {
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"LANG": "de_DE.UTF-8"}, "de-de"},
		{map[string]string{"LC_ALL": "zh_TW.UTF-8", "LANG": "fr_FR"}, "zh-tw"},
		{map[string]string{"LANG": "zh_CN"}, "zh-hans"},
		{map[string]string{"LANG": "pt_PT"}, "pt-br"},
		{map[string]string{"LANG": "C"}, "en-us"},
		{nil, "en-us"},
	} {
		require.Equal(t, c.want, supportLanguage(env(c.env)), c.env)
	}
}

func TestCountryAlpha3(t *testing.T) {
	for in, want := range map[string]string{"US": "USA", "gb": "GBR", "CAN": "CAN"} {
		got, ok := countryAlpha3(in)
		require.True(t, ok, in)
		require.Equal(t, want, got, in)
	}
	for _, in := range []string{"XX", "UK", "EZ", "DD"} {
		got, ok := countryAlpha3(in)
		require.False(t, ok, in)
		require.Empty(t, got, in)
	}
}

func TestCountryOptions(t *testing.T) {
	opts := countryOptions()
	require.Contains(t, opts, "Canada (CAN)")
	seen := map[string]bool{}
	for _, o := range opts {
		require.NotContains(t, o, "(ZZZ)")
		code := o[len(o)-4 : len(o)-1]
		require.False(t, seen[code], o)
		seen[code] = true
	}
}

func TestContactFromMe(t *testing.T) {
	full := me{GivenName: "Ada", Surname: "Lovelace", Mail: "ada@x.com", UserPrincipalName: "ada@y.com", UsageLocation: "CA"}
	e := env(map[string]string{"LANG": "fr_FR.UTF-8"})
	require.Equal(t, contactDefaults{"Ada", "Lovelace", "ada@x.com", "CAN", "Pacific Standard Time", "fr-fr"},
		contactFromMe(full, e, "America/Los_Angeles"))

	m := full
	m.Mail, m.UserPrincipalName = "", "a@b.com"
	require.Equal(t, "a@b.com", contactFromMe(m, e, "").Email)
	m.UserPrincipalName = "a_b.com#EXT#@t.onmicrosoft.com"
	require.Equal(t, "", contactFromMe(m, e, "").Email)
	m.UsageLocation = ""
	require.Equal(t, "", contactFromMe(m, e, "").Country)
}

type fakeCred struct{}

func (fakeCred) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "tok", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func graphServer(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	old := graphBaseURL
	graphBaseURL = srv.URL
	t.Cleanup(func() { graphBaseURL = old; srv.Close() })
}

func TestFetchMe(t *testing.T) {
	graphServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1.0/me", r.URL.Path)
		require.Equal(t, "givenName,surname,mail,userPrincipalName,usageLocation", r.URL.Query().Get("$select"))
		require.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"givenName":"Ada","surname":"L","mail":null,"userPrincipalName":"u@x","usageLocation":"CA"}`))
	})
	m, err := fetchMe(context.Background(), fakeCred{})
	require.NoError(t, err)
	require.Equal(t, me{GivenName: "Ada", Surname: "L", UserPrincipalName: "u@x", UsageLocation: "CA"}, m)
}

func TestFetchMeError(t *testing.T) {
	graphServer(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "denied", http.StatusForbidden) })
	_, err := fetchMe(context.Background(), fakeCred{})
	require.ErrorContains(t, err, "403")
}
