package ado

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// newTestClient builds a Client against srv with a hermetic, network-free
// auth path: AAD is forced to fail (no real az login/network dependency in
// tests) and a fake PAT supplies the credential instead.
func newTestClient(t *testing.T, org string) *Client {
	t.Helper()
	t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
	t.Setenv("AZURE_DEVOPS_EXT_PAT", "test-pat")

	orig := getCredential
	getCredential = func() (azcore.TokenCredential, error) {
		return nil, errors.New("no AAD in test")
	}
	t.Cleanup(func() { getCredential = orig })

	c, err := NewClient(context.Background(), org)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestURLBuilding(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want string
	}{
		{
			name: "org-scoped",
			req:  Request{Path: "git/pullRequests/42", APIVersion: "5.0"},
			want: "/myorg/_apis/git/pullRequests/42?api-version=5.0",
		},
		{
			name: "project-scoped",
			req:  Request{Scope: "MyProj", Path: "git/repositories", APIVersion: "5.0"},
			want: "/myorg/MyProj/_apis/git/repositories?api-version=5.0",
		},
		{
			name: "team-scoped",
			req:  Request{Scope: "MyProj/MyTeam", Path: "work/teamsettings", APIVersion: "5.0"},
			want: "/myorg/MyProj/MyTeam/_apis/work/teamsettings?api-version=5.0",
		},
		{
			name: "collection-scoped",
			req:  Request{Scope: "DefaultCollection/MyProject", Path: "git/repositories", APIVersion: "5.0"},
			want: "/myorg/DefaultCollection/MyProject/_apis/git/repositories?api-version=5.0",
		},
		{
			name: "project name with a space",
			req:  Request{Scope: "My Proj", Path: "git/repositories", APIVersion: "5.0"},
			want: "/myorg/My%20Proj/_apis/git/repositories?api-version=5.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.String()
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("{}"))
			}))
			defer srv.Close()

			c := newTestClient(t, srv.URL+"/myorg")
			if err := c.Do(context.Background(), tt.req, nil); err != nil {
				t.Fatalf("Do: %v", err)
			}
			if got != tt.want {
				t.Errorf("got URL %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFedAuthAndMsaPassThroughHeaders(t *testing.T) {
	var gotFedAuth, gotMsaPassThrough string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFedAuth = r.Header.Get("X-TFS-FedAuthRedirect")
		gotMsaPassThrough = r.Header.Get("X-VSS-ForceMsaPassThrough")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL+"/myorg")
	if err := c.Do(context.Background(), Request{Path: "git/repositories", APIVersion: "5.0"}, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotFedAuth != "Suppress" {
		t.Errorf("X-TFS-FedAuthRedirect = %q, want %q", gotFedAuth, "Suppress")
	}
	if gotMsaPassThrough != "true" {
		t.Errorf("X-VSS-ForceMsaPassThrough = %q, want %q", gotMsaPassThrough, "true")
	}
}

func TestHostFor(t *testing.T) {
	tests := []struct {
		host, sub, want string
	}{
		{"dev.azure.com", "vsrm", "vsrm.dev.azure.com"},
		{"myorg.visualstudio.com", "vssps", "myorg.vssps.visualstudio.com"},
		{"dev.azure.com", "", "dev.azure.com"},
	}
	for _, tt := range tests {
		if got := hostFor(tt.host, tt.sub); got != tt.want {
			t.Errorf("hostFor(%q, %q) = %q, want %q", tt.host, tt.sub, got, tt.want)
		}
	}
}

func TestListFollowsContinuationToken(t *testing.T) {
	t.Run("two pages", func(t *testing.T) {
		calls := 0
		var sawContinuationToken string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			if calls == 1 {
				w.Header().Set("X-MS-ContinuationToken", "tok1")
				w.Write([]byte(`{"count":1,"value":[{"id":"a"}]}`))
				return
			}
			sawContinuationToken = r.URL.Query().Get("continuationToken")
			w.Write([]byte(`{"count":1,"value":[{"id":"b"}]}`))
		}))
		defer srv.Close()

		c := newTestClient(t, srv.URL+"/myorg")
		var out []map[string]any
		if err := c.List(context.Background(), Request{Path: "git/repositories", APIVersion: "5.0"}, &out); err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(out) != 2 {
			t.Errorf("got %d items, want 2", len(out))
		}
		if calls != 2 {
			t.Errorf("got %d requests, want 2", calls)
		}
		if sawContinuationToken != "tok1" {
			t.Errorf("second request continuationToken = %q, want %q", sawContinuationToken, "tok1")
		}
	})

	t.Run("repeated token terminates", func(t *testing.T) {
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-MS-ContinuationToken", "same")
			w.Write([]byte(`{"count":1,"value":[{"id":"a"}]}`))
		}))
		defer srv.Close()

		c := newTestClient(t, srv.URL+"/myorg")
		var out []map[string]any
		if err := c.List(context.Background(), Request{Path: "git/repositories", APIVersion: "5.0"}, &out); err != nil {
			t.Fatalf("List: %v", err)
		}
		if calls != 2 {
			t.Errorf("got %d requests, want 2 (loop must terminate on repeated token)", calls)
		}
		if len(out) != 2 {
			t.Errorf("got %d items, want 2", len(out))
		}
	})
}

func TestListDoesNotFollowTokenWhenTopIsSet(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-MS-ContinuationToken", "tok1")
		w.Write([]byte(`{"count":1,"value":[{"id":"a"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL+"/myorg")
	var out []map[string]any
	q := url.Values{}
	q.Set("$top", "5")
	if err := c.List(context.Background(), Request{Path: "git/repositories", APIVersion: "5.0", Query: q}, &out); err != nil {
		t.Fatalf("List: %v", err)
	}
	if calls != 1 {
		t.Errorf("got %d requests, want 1 (List must not follow the continuation token when $top is set)", calls)
	}
	if len(out) != 1 {
		t.Errorf("got %d items, want 1", len(out))
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
		wantMessage string
		wantTypeKey string
	}{
		{
			name:        "wrapped exception",
			status:      404,
			contentType: "application/json",
			body:        `{"$id":"1","innerException":null,"message":"TF200016: The following project does not exist: Foo. Verify that the name of the project is correct and that the project exists on the specified Azure DevOps Server.","typeName":"Microsoft.TeamFoundation.Core.WebApi.ProjectDoesNotExistWithNameException, Microsoft.TeamFoundation.Core.WebApi","typeKey":"ProjectDoesNotExistWithNameException","errorCode":0,"eventId":3000}`,
			wantMessage: "TF200016: The following project does not exist: Foo. Verify that the name of the project is correct and that the project exists on the specified Azure DevOps Server.",
			wantTypeKey: "ProjectDoesNotExistWithNameException",
		},
		{
			// ImproperException is only tried against a {"count":N,"value":
			// {...}} collection wrapper's unwrapped value (client.py:252-257)
			// — never against a raw, non-collection body.
			name:        "improper exception",
			status:      500,
			contentType: "application/json",
			body:        `{"count":1,"value":{"Message":"boom"}}`,
			wantMessage: "boom",
		},
		{
			// A raw (non-collection) body with a PascalCase "Message" key —
			// the SystemException shape. (encoding/json matches JSON keys to
			// struct tags case-insensitively, so this also satisfies the
			// WrappedException struct; either way the extracted message is
			// the same and TypeKey is empty, since there is no "typeKey".)
			name:        "system exception",
			status:      500,
			contentType: "application/json",
			body:        `{"ClassName":"System.Exception","Message":"crashed"}`,
			wantMessage: "crashed",
		},
		{
			// text/plain always falls through to the bottom formatting
			// (client.py:263-264,270), which bakes in a two-space separator
			// and always appends "Operation returned a N status code." —
			// unlike a matched JSON exception, which raises immediately with
			// just its own message and no suffix.
			name:        "text/plain",
			status:      400,
			contentType: "text/plain",
			body:        "nope",
			wantMessage: "nope  Operation returned a 400 status code.",
		},
		{
			// A missing Content-Type goes down the JSON path, not the
			// text/plain path (client.py:243: `content_type is None or
			// content_type.find('text/plain') < 0`).
			name:        "missing content-type parses as JSON",
			status:      404,
			contentType: "",
			body:        `{"message":"gone","typeKey":"NotFoundException"}`,
			wantMessage: "gone",
			wantTypeKey: "NotFoundException",
		},
		{
			name:        "empty body",
			status:      500,
			contentType: "application/json",
			body:        "",
			wantMessage: "Operation returned a 500 status code.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newAPIError(tt.status, tt.contentType, []byte(tt.body), "https://dev.azure.com/myorg/_apis/x")
			if e.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", e.Message, tt.wantMessage)
			}
			if e.TypeKey != tt.wantTypeKey {
				t.Errorf("TypeKey = %q, want %q", e.TypeKey, tt.wantTypeKey)
			}
		})
	}

	t.Run("401 appends auth message", func(t *testing.T) {
		url := "https://dev.azure.com/myorg/_apis/x"
		e := newAPIError(401, "application/json", nil, url)
		want := "The requested resource requires user authentication: " + url
		if e.Message[len(e.Message)-len(want):] != want {
			t.Errorf("Message = %q, want suffix %q", e.Message, want)
		}
	})

	t.Run("401 text/plain keeps the two-space separator", func(t *testing.T) {
		// client.py:263-266: error_message = body + "  ", then
		// "{error_message}The requested resource requires user
		// authentication: {url}" — the separator is baked into
		// error_message, not appended between it and the phrase.
		url := "https://dev.azure.com/myorg/_apis/x"
		e := newAPIError(401, "text/plain", []byte("denied"), url)
		want := "denied  The requested resource requires user authentication: " + url
		if e.Message != want {
			t.Errorf("Message = %q, want %q", e.Message, want)
		}
	})
}

func TestJSONPatchContentType(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL+"/myorg")

	if err := c.Do(context.Background(), Request{
		Method:     http.MethodPatch,
		Path:       "wit/workItems/1",
		APIVersion: "5.0",
		Body:       []any{map[string]any{"op": "add"}},
		JSONPatch:  true,
	}, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotContentType != "application/json-patch+json" {
		t.Errorf("Content-Type = %q, want application/json-patch+json", gotContentType)
	}

	if err := c.Do(context.Background(), Request{
		Method:     http.MethodPost,
		Path:       "wit/workItems",
		APIVersion: "5.0",
		Body:       map[string]any{"a": 1},
	}, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotContentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", gotContentType)
	}
}

func TestMissingAPIVersionErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request must not be sent when APIVersion is missing")
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL+"/myorg")
	err := c.Do(context.Background(), Request{Path: "git/repositories"}, nil)
	if err == nil {
		t.Fatal("Do: want error for missing APIVersion, got nil")
	}
}
