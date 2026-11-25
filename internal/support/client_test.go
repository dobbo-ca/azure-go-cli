package support

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
	"github.com/stretchr/testify/require"
)

type staticCred struct{}

func (staticCred) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "tok", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

// newTestClient serves handler over TLS and returns a Client for scope s.
func newTestClient(t *testing.T, s Scope, handler http.HandlerFunc) Client {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	opts := &arm.ClientOptions{ClientOptions: policy.ClientOptions{
		Cloud: cloud.Configuration{Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
			cloud.ResourceManager: {Endpoint: srv.URL, Audience: "https://management.azure.com"},
		}},
		Transport: srv.Client(),
	}}
	c, err := newClient(s, staticCred{}, opts)
	require.NoError(t, err)
	return c
}

func TestSubscriptionClientListTicketsPaths(t *testing.T) {
	var mu sync.Mutex
	var reqs []*http.Request
	c := newTestClient(t, Scope{SubscriptionID: "sub1"}, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs = append(reqs, r)
		mu.Unlock()
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `{"value":[{"name":"t2"}]}`)
			return
		}
		fmt.Fprintf(w, `{"value":[{"name":"t1"}],"nextLink":"https://%s%s?api-version=2024-04-01&page=2"}`, r.Host, r.URL.Path)
	})
	got, err := c.ListTickets(context.Background(), "Status eq 'Open'")
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "/subscriptions/sub1/providers/Microsoft.Support/supportTickets", reqs[0].URL.Path)
	require.Equal(t, "Status eq 'Open'", reqs[0].URL.Query().Get("$filter"))
}

func TestTenantClientListTicketsPath(t *testing.T) {
	var path string
	c := newTestClient(t, Scope{TenantID: "t1"}, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		fmt.Fprint(w, `{"value":[]}`)
	})
	_, err := c.ListTickets(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, "/providers/Microsoft.Support/supportTickets", path)
}

func TestCheckNameUnavailableReturnsMessage(t *testing.T) {
	c := newTestClient(t, Scope{SubscriptionID: "sub1"}, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"nameAvailable":false,"message":"Name taken"}`)
	})
	err := c.CheckName(context.Background(), "x", armsupport.TypeMicrosoftSupportSupportTickets)
	require.EqualError(t, err, "Name taken")
}
