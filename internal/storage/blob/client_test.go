package blob

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const emptyListXML = `<?xml version="1.0" encoding="utf-8"?><EnumerationResults ContainerName="c"><Blobs></Blobs><NextMarker/></EnumerationResults>`

// TestBlobListCredentials runs `blob list` against a fake blob endpoint and
// checks which credential reaches the wire.
func TestBlobListCredentials(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("not-a-real-key"))
	cases := []struct {
		name  string
		args  []string
		check func(r *http.Request) string
	}{
		{
			name: "account key signs with SharedKey",
			args: []string{"--account-name", "acct", "--account-key", key},
			check: func(r *http.Request) string {
				if !strings.HasPrefix(r.Header.Get("Authorization"), "SharedKey acct:") {
					return "Authorization = " + r.Header.Get("Authorization")
				}
				return ""
			},
		},
		{
			name: "account name comes from the endpoint",
			args: []string{"--account-key", key},
			check: func(r *http.Request) string {
				if !strings.HasPrefix(r.Header.Get("Authorization"), "SharedKey acct:") {
					return "Authorization = " + r.Header.Get("Authorization")
				}
				return ""
			},
		},
		{
			name: "sas token goes in the query",
			args: []string{"--account-name", "acct", "--sas-token", "?sv=2022-11-02&sig=abc"},
			check: func(r *http.Request) string {
				if r.URL.Query().Get("sig") != "abc" || r.Header.Get("Authorization") != "" {
					return "query = " + r.URL.RawQuery + ", Authorization = " + r.Header.Get("Authorization")
				}
				return ""
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearAuthEnv(t)
			var problem string
			var hits int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				problem = tc.check(r)
				w.Header().Set("Content-Type", "application/xml")
				w.Write([]byte(emptyListXML))
			}))
			defer srv.Close()

			cmd := NewBlobCommand()
			cmd.PersistentFlags().StringP("output", "o", "json", "")
			cmd.PersistentFlags().String("query", "", "")
			cmd.SetArgs(append([]string{"list", "-c", "c", "--blob-endpoint", srv.URL + "/acct", "-o", "json"}, tc.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if hits == 0 {
				t.Fatal("no request reached the server")
			}
			if problem != "" {
				t.Error(problem)
			}
		})
	}
}
