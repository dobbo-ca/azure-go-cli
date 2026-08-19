package devops

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
)

// securityTestClient builds an *ado.Client whose HTTP transport dials srv
// regardless of the host in the request URL — needed because several of
// these commands set Request.Host: "vssps", which ado.Client rewrites to a
// subdomain (e.g. "vssps.dev.azure.com") that would not otherwise resolve to
// the httptest server (mirrors extensionTestClient in extension_test.go).
func securityTestClient(srv *httptest.Server) *ado.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return net.Dial(network, srv.Listener.Addr().String())
		},
	}
	return &ado.Client{Org: "http://dev.azure.com/myorg", HTTP: &http.Client{Transport: transport}}
}

func securityWithTestClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := securityNewClient
	securityNewClient = func(ctx context.Context, org string) (*ado.Client, error) {
		return securityTestClient(srv), nil
	}
	t.Cleanup(func() { securityNewClient = orig })
}

func securityDecodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	return body
}

// TestSecurityFirstACE_Deterministic guards B8: ranging a Go map randomizes
// iteration order per run, so a multi-ACE ACL's Effective Allow/Deny would
// vary between identical runs; picking the lowest descriptor key keeps it
// stable.
func TestSecurityFirstACE_Deterministic(t *testing.T) {
	row := map[string]any{
		"acesDictionary": map[string]any{
			"vssgp.zzz": map[string]any{"allow": 1},
			"vssgp.aaa": map[string]any{"allow": 2},
		},
	}
	for i := 0; i < 20; i++ {
		got := securityFirstACE(row)
		if got["allow"] != 2 {
			t.Fatalf("iteration %d: got allow=%v, want 2 (vssgp.aaa, lowest key)", i, got["allow"])
		}
	}
}

// TestSecurityGroupMembershipAdd_RequestSequence covers add_membership's
// 3-call sequence (security_group.py:175-199): the PUT establishing the
// edge takes no body, and the SubjectLookup hydration afterward requests
// the container before the member — which the table output then renders as
// 2 rows (group, then member), unlike list's 4-column row shape.
func TestSecurityGroupMembershipAdd_RequestSequence(t *testing.T) {
	var calls []string
	var lookupBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/myorg/_apis/Graph/Memberships/vssgp.member/vssgp.group":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"containerDescriptor": "vssgp.group",
				"memberDescriptor":    "vssgp.member",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/myorg/_apis/Graph/SubjectLookup":
			lookupBody = securityDecodeBody(t, r)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"vssgp.group":  map[string]any{"subjectKind": "group", "principalName": "MyGroup", "mailAddress": "", "descriptor": "vssgp.group"},
				"vssgp.member": map[string]any{"subjectKind": "user", "displayName": "A User", "mailAddress": "a@example.com", "descriptor": "vssgp.member"},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	securityWithTestClient(t, srv)

	cmd := securityGroupMembershipAddCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("detect", "false")
	cmd.Flags().Set("member-id", "vssgp.member")
	cmd.Flags().Set("group-id", "vssgp.group")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")

	if err := securityGroupMembershipAddRun(context.Background(), cmd); err != nil {
		t.Fatalf("securityGroupMembershipAddRun: %v", err)
	}

	if len(calls) != 2 || calls[0] != "PUT /myorg/_apis/Graph/Memberships/vssgp.member/vssgp.group" || calls[1] != "POST /myorg/_apis/Graph/SubjectLookup" {
		t.Fatalf("calls = %v, want [PUT .../vssgp.member/vssgp.group, POST .../SubjectLookup]", calls)
	}

	keys, _ := lookupBody["lookupKeys"].([]any)
	if len(keys) != 2 {
		t.Fatalf("lookupKeys = %v, want 2 entries", keys)
	}
	first := keys[0].(map[string]any)
	second := keys[1].(map[string]any)
	if first["descriptor"] != "vssgp.group" || second["descriptor"] != "vssgp.member" {
		t.Errorf("lookupKeys order = [%v, %v], want [vssgp.group, vssgp.member] (container before member)", first["descriptor"], second["descriptor"])
	}
}

// TestSecurityGroupMembershipAdd_JSONReturnsDescriptorKeyedMap guards B5:
// add_membership (security_group.py:198) returns lookup_subjects'
// descriptor-keyed dict verbatim, not an array built from ranging a Go map.
func TestSecurityGroupMembershipAdd_JSONReturnsDescriptorKeyedMap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"containerDescriptor": "vssgp.group",
				"memberDescriptor":    "vssgp.member",
			})
		case r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"vssgp.group":  map[string]any{"subjectKind": "group", "principalName": "MyGroup"},
				"vssgp.member": map[string]any{"subjectKind": "user", "displayName": "A User"},
			})
		}
	}))
	defer srv.Close()

	securityWithTestClient(t, srv)

	cmd := securityGroupMembershipAddCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("detect", "false")
	cmd.Flags().Set("member-id", "vssgp.member")
	cmd.Flags().Set("group-id", "vssgp.group")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")

	out := teamCaptureStdout(t, func() {
		if err := securityGroupMembershipAddRun(context.Background(), cmd); err != nil {
			t.Fatalf("securityGroupMembershipAddRun: %v", err)
		}
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not a JSON object: %v\n%s", err, out)
	}
	if len(got) != 2 || got["vssgp.group"] == nil || got["vssgp.member"] == nil {
		t.Fatalf("got = %v, want a descriptor-keyed object with vssgp.group and vssgp.member", got)
	}
}

// TestSecurityGroupMembershipRemove_HeadThenDelete covers
// remove_membership's existence-check-then-delete (security_group.py:215-222):
// a HEAD 404 becomes the friendlier CLIError and the DELETE is never issued.
func TestSecurityGroupMembershipRemove_HeadThenDelete(t *testing.T) {
	tests := []struct {
		name       string
		headStatus int
		wantCalls  []string
		wantErr    string
	}{
		{
			name:       "exists",
			headStatus: http.StatusOK,
			wantCalls:  []string{"HEAD", "DELETE"},
		},
		{
			name:       "missing",
			headStatus: http.StatusNotFound,
			wantCalls:  []string{"HEAD"},
			wantErr:    "Membership doesn't exists.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method)
				if r.URL.Path != "/myorg/_apis/Graph/Memberships/vssgp.member/vssgp.group" {
					t.Errorf("path = %s", r.URL.Path)
				}
				if r.Method == http.MethodHead {
					w.WriteHeader(tt.headStatus)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			securityWithTestClient(t, srv)

			cmd := securityGroupMembershipRemoveCmd()
			cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
			cmd.Flags().Set("detect", "false")
			cmd.Flags().Set("member-id", "vssgp.member")
			cmd.Flags().Set("group-id", "vssgp.group")
			cmd.Flags().Set("yes", "true")
			cmd.Flags().String("output", "json", "")
			cmd.Flags().String("query", "", "")

			err := securityGroupMembershipRemoveRun(context.Background(), cmd)

			if len(calls) != len(tt.wantCalls) {
				t.Fatalf("calls = %v, want %v", calls, tt.wantCalls)
			}
			for i, want := range tt.wantCalls {
				if calls[i] != want {
					t.Errorf("calls[%d] = %s, want %s", i, calls[i], want)
				}
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			} else if err == nil || err.Error() != tt.wantErr {
				t.Errorf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

// TestSecurityPermissionUpdate_RequestSequence covers update_permissions'
// 4-call sequence (security_permission.py:93-119): the POST body shape and
// the local "deny wins for overlapping bits" narrowing of which permission
// rows get echoed back (changed_bits = (allow &^ deny) + deny), without
// altering what was actually sent to the server.
func TestSecurityPermissionUpdate_RequestSequence(t *testing.T) {
	var setBody map[string]any
	var calls []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/myorg/_apis/Identities":
			// "vssgp.subject" contains a '.', so update_permissions's
			// subject resolution (security_permission.py:196-198) routes it
			// through get_identity_descriptor_from_subject_descriptor.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]any{{"descriptor": "vssgp.subject"}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/myorg/_apis/AccessControlEntries/ns1":
			setBody = securityDecodeBody(t, r)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/myorg/_apis/AccessControlLists/ns1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"count": 1,
				"value": []map[string]any{
					{
						"token":               "tok1",
						"includeExtendedInfo": true,
						"acesDictionary": map[string]any{
							"vssgp.subject": map[string]any{
								"allow": 3, "deny": 1,
								"extendedInfo": map[string]any{"effectiveAllow": 2, "effectiveDeny": 1},
							},
						},
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/myorg/_apis/SecurityNamespaces/ns1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"count": 1,
				"value": []map[string]any{
					{"actions": []map[string]any{
						{"bit": 1, "name": "Read", "displayName": "Read"},
						{"bit": 2, "name": "Write", "displayName": "Write"},
					}},
				},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	securityWithTestClient(t, srv)

	cmd := securityPermissionUpdateCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("detect", "false")
	cmd.Flags().Set("namespace-id", "ns1")
	cmd.Flags().Set("subject", "vssgp.subject")
	cmd.Flags().Set("token", "tok1")
	cmd.Flags().Set("allow-bit", "3")
	cmd.Flags().Set("deny-bit", "1")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")

	if err := securityPermissionUpdateRun(context.Background(), cmd, nil); err != nil {
		t.Fatalf("securityPermissionUpdateRun: %v", err)
	}

	wantCalls := []string{
		"GET /myorg/_apis/Identities",
		"POST /myorg/_apis/AccessControlEntries/ns1",
		"GET /myorg/_apis/AccessControlLists/ns1",
		"GET /myorg/_apis/SecurityNamespaces/ns1",
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %v, want %v", calls, wantCalls)
	}
	for i, want := range wantCalls {
		if calls[i] != want {
			t.Errorf("calls[%d] = %s, want %s", i, calls[i], want)
		}
	}

	aces, _ := setBody["accessControlEntries"].([]any)
	if len(aces) != 1 {
		t.Fatalf("accessControlEntries = %v, want 1 entry", aces)
	}
	ace := aces[0].(map[string]any)
	// allow=3, deny=1 sent as-is to the server, unmodified by the local
	// display-only narrowing done afterward.
	if ace["allow"] != float64(3) || ace["deny"] != float64(1) || ace["descriptor"] != "vssgp.subject" {
		t.Errorf("ace = %v, want allow=3 deny=1 descriptor=vssgp.subject", ace)
	}
	if setBody["token"] != "tok1" || setBody["merge"] != true {
		t.Errorf("token/merge = %v/%v, want tok1/true", setBody["token"], setBody["merge"])
	}
}

// TestSecurityPermissionUpdate_MergeSpaceForm guards B6: "--merge false"
// (space-separated) leaves "false" as a stray positional that RunE must
// fold back in, rather than binding --merge to its NoOptDefVal "true" and
// silently dropping "false".
func TestSecurityPermissionUpdate_MergeSpaceForm(t *testing.T) {
	var setBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/myorg/_apis/Identities":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]any{{"descriptor": "vssgp.subject"}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/myorg/_apis/AccessControlEntries/ns1":
			setBody = securityDecodeBody(t, r)
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/myorg/_apis/AccessControlLists/ns1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"count": 1,
				"value": []map[string]any{
					{"token": "tok1", "acesDictionary": map[string]any{"vssgp.subject": map[string]any{"allow": 1, "deny": 0}}},
				},
			})
		case r.URL.Path == "/myorg/_apis/SecurityNamespaces/ns1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"count": 1,
				"value": []map[string]any{{"actions": []map[string]any{{"bit": 1, "name": "Read", "displayName": "Read"}}}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	securityWithTestClient(t, srv)

	cmd := securityPermissionUpdateCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("detect", "false")
	cmd.Flags().Set("namespace-id", "ns1")
	cmd.Flags().Set("subject", "vssgp.subject")
	cmd.Flags().Set("token", "tok1")
	cmd.Flags().Set("allow-bit", "1")
	// Bare "--merge" binds to NoOptDefVal "true"; the space-separated
	// "false" is what RunE must pick up from args.
	cmd.Flags().Set("merge", "true")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")

	if err := securityPermissionUpdateRun(context.Background(), cmd, []string{"false"}); err != nil {
		t.Fatalf("securityPermissionUpdateRun: %v", err)
	}
	if setBody["merge"] != false {
		t.Errorf("merge = %v, want false", setBody["merge"])
	}
}

// TestSecurityPermissionShow_JSONReturnsFullACL guards B2: _update_json
// (security_permission.py:171-178) returns the whole ACL list with
// resolvedPermissions spliced into each ace, not the bits array alone — so
// token/allow/deny/extendedInfo/acesDictionary must survive in -o json.
func TestSecurityPermissionShow_JSONReturnsFullACL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/myorg/_apis/Identities":
			// "vssgp.subject" contains a '.', routing through
			// get_identity_descriptor_from_subject_descriptor.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]any{{"descriptor": "vssgp.subject"}},
			})
		case r.URL.Path == "/myorg/_apis/AccessControlLists/ns1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"count": 1,
				"value": []map[string]any{
					{
						"token":               "tok1",
						"includeExtendedInfo": true,
						"acesDictionary": map[string]any{
							"vssgp.subject": map[string]any{"allow": 3, "deny": 1},
						},
					},
				},
			})
		case r.URL.Path == "/myorg/_apis/SecurityNamespaces/ns1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"count": 1,
				"value": []map[string]any{
					{"actions": []map[string]any{{"bit": 1, "name": "Read", "displayName": "Read"}}},
				},
			})
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	securityWithTestClient(t, srv)

	cmd := securityPermissionShowCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("detect", "false")
	cmd.Flags().Set("namespace-id", "ns1")
	cmd.Flags().Set("subject", "vssgp.subject")
	cmd.Flags().Set("token", "tok1")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")

	out := teamCaptureStdout(t, func() {
		if err := securityPermissionShowRun(context.Background(), cmd); err != nil {
			t.Fatalf("securityPermissionShowRun: %v", err)
		}
	})

	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not valid JSON array: %v\n%s", err, out)
	}
	if len(got) != 1 || got[0]["token"] != "tok1" {
		t.Fatalf("got = %v, want a 1-element ACL list with token=tok1", got)
	}
	acesDict, _ := got[0]["acesDictionary"].(map[string]any)
	ace, _ := acesDict["vssgp.subject"].(map[string]any)
	resolved, _ := ace["resolvedPermissions"].([]any)
	if len(resolved) != 1 {
		t.Fatalf("acesDictionary[vssgp.subject].resolvedPermissions = %v, want 1 entry", ace["resolvedPermissions"])
	}
}

// TestSecurityPermissionNamespaceShow_JSONReturnsFullNamespaceList guards
// B3: show_namespace (security_permission.py:28-33) returns the full
// [SecurityNamespaceDescription] list verbatim, not just its actions array.
func TestSecurityPermissionNamespaceShow_JSONReturnsFullNamespaceList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"count": 1,
			"value": []map[string]any{
				{
					"namespaceId": "ns1",
					"name":        "Git Repositories",
					"displayName": "Git Repositories",
					"actions":     []map[string]any{{"bit": 1, "name": "Read", "displayName": "Read"}},
				},
			},
		})
	}))
	defer srv.Close()

	securityWithTestClient(t, srv)

	cmd := securityPermissionNamespaceShowCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("detect", "false")
	cmd.Flags().Set("namespace-id", "ns1")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")

	out := teamCaptureStdout(t, func() {
		if err := securityPermissionNamespaceShowRun(context.Background(), cmd); err != nil {
			t.Fatalf("securityPermissionNamespaceShowRun: %v", err)
		}
	})

	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not valid JSON array: %v\n%s", err, out)
	}
	if len(got) != 1 || got[0]["namespaceId"] != "ns1" || got[0]["displayName"] != "Git Repositories" {
		t.Fatalf("got = %v, want the full namespace object (namespaceId/displayName present)", got)
	}
	if _, ok := got[0]["actions"]; !ok {
		t.Error("expected actions to still be present in the full namespace object")
	}
}

// TestSecurityResolveMemberDescriptor covers the '@'-or-no-'.' heuristic
// shared by group membership list/add/remove (security_group.py:156,187,212):
// a descriptor (contains '.') passes straight through with no HTTP calls; an
// email routes through identity resolution then a descriptor lookup.
func TestSecurityResolveMemberDescriptor(t *testing.T) {
	t.Run("descriptor passthrough", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}))
		defer srv.Close()

		got, err := securityResolveMemberDescriptor(context.Background(), securityTestClient(srv), "vssgp.Uy0xLTk")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if got != "vssgp.Uy0xLTk" {
			t.Errorf("got %q, want passthrough", got)
		}
	})

	t.Run("email resolves via identities then descriptor", func(t *testing.T) {
		var paths []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/myorg/_apis/Identities":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"value": []map[string]any{{"id": "id-123", "descriptor": "Microsoft.IdentityModel.Claims.ClaimsIdentity;id-123"}},
				})
			case "/myorg/_apis/Graph/Descriptors/id-123":
				_ = json.NewEncoder(w).Encode(map[string]any{"value": "aad.abc123"})
			default:
				t.Errorf("unexpected request: %s", r.URL.Path)
			}
		}))
		defer srv.Close()

		got, err := securityResolveMemberDescriptor(context.Background(), securityTestClient(srv), "user@example.com")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if got != "aad.abc123" {
			t.Errorf("got %q, want aad.abc123", got)
		}
		if len(paths) != 2 {
			t.Fatalf("calls = %v, want 2", paths)
		}
	})
}

// TestSecurityGroupCreate_MutualExclusivity covers create_group's
// if/elif/else validation (security_group.py:82-89), which must error on
// the else branch (including when none of the three are set) rather than
// on any smarter combinatorial check.
func TestSecurityGroupCreate_MutualExclusivity(t *testing.T) {
	tests := []struct {
		name    string
		set     map[string]string
		wantErr bool
	}{
		{"name only", map[string]string{"name": "Readers"}, false},
		{"origin only", map[string]string{"origin-id": "origin1"}, false},
		{"email only", map[string]string{"email-id": "a@b.com"}, false},
		{"name and origin", map[string]string{"name": "Readers", "origin-id": "origin1"}, true},
		{"none set", map[string]string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBody = securityDecodeBody(t, r)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"principalName": "x", "description": ""})
			}))
			defer srv.Close()

			securityWithTestClient(t, srv)

			cmd := securityGroupCreateCmd()
			cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
			cmd.Flags().Set("detect", "false")
			cmd.Flags().Set("scope", "organization")
			for k, v := range tt.set {
				cmd.Flags().Set(k, v)
			}
			cmd.Flags().String("output", "json", "")
			cmd.Flags().String("query", "", "")

			err := securityGroupCreateRun(context.Background(), cmd)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			switch {
			case tt.set["name"] != "":
				if gotBody["displayName"] != tt.set["name"] {
					t.Errorf("body = %v, want displayName=%s", gotBody, tt.set["name"])
				}
			case tt.set["origin-id"] != "":
				if gotBody["originId"] != tt.set["origin-id"] {
					t.Errorf("body = %v, want originId=%s", gotBody, tt.set["origin-id"])
				}
			case tt.set["email-id"] != "":
				if gotBody["mailAddress"] != tt.set["email-id"] {
					t.Errorf("body = %v, want mailAddress=%s", gotBody, tt.set["email-id"])
				}
			}
		})
	}
}

// TestSecurityGroupList_JSONEnvelope guards B4: list_groups returns
// PagedGraphGroups (graphGroups/continuationToken, models.py:380-392), not
// a bare array — every --query "graphGroups[?...]" broke against a bare
// array.
func TestSecurityGroupList_JSONEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"count": 1, "value": []map[string]any{{"principalName": "Readers"}}})
	}))
	defer srv.Close()

	securityWithTestClient(t, srv)

	cmd := securityGroupListCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("detect", "false")
	cmd.Flags().Set("scope", "organization")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")

	out := teamCaptureStdout(t, func() {
		if err := securityGroupListRun(context.Background(), cmd); err != nil {
			t.Fatalf("securityGroupListRun: %v", err)
		}
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["continuationToken"]; !ok {
		t.Error("envelope missing continuationToken key")
	}
	groups, ok := got["graphGroups"].([]any)
	if !ok || len(groups) != 1 {
		t.Errorf("graphGroups = %v", got["graphGroups"])
	}
}

// TestSecurityGroupCreate_OrganizationScopeKeepsExplicitProject guards
// AUTH-02/B1: security_group.py:75-80's org branch only resolves the
// organization (resolve_instance) — project stays exactly the raw --project
// value, so an explicit --project alongside --scope organization still
// scopes the created group.
func TestSecurityGroupCreate_OrganizationScopeKeepsExplicitProject(t *testing.T) {
	var gotScopeDescriptor string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/myorg/_apis/projects/MyProj":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "proj-guid-1"})
		case r.URL.Path == "/myorg/_apis/Graph/Descriptors/proj-guid-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"value": "scope-descriptor-1"})
		case r.URL.Path == "/myorg/_apis/Graph/Groups":
			gotScopeDescriptor = r.URL.Query().Get("scopeDescriptor")
			_ = json.NewEncoder(w).Encode(map[string]any{"principalName": "x"})
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	securityWithTestClient(t, srv)

	cmd := securityGroupCreateCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("detect", "false")
	cmd.Flags().Set("scope", "organization")
	cmd.Flags().Set("project", "MyProj")
	cmd.Flags().Set("name", "Readers")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")

	if err := securityGroupCreateRun(context.Background(), cmd); err != nil {
		t.Fatalf("securityGroupCreateRun: %v", err)
	}
	if gotScopeDescriptor != "scope-descriptor-1" {
		t.Errorf("scopeDescriptor = %q, want scope-descriptor-1 (explicit --project kept for org scope)", gotScopeDescriptor)
	}
}

// TestSecurityGroupList_OrganizationScopeKeepsExplicitProject is
// TestSecurityGroupCreate_OrganizationScopeKeepsExplicitProject's
// counterpart for `security group list`.
func TestSecurityGroupList_OrganizationScopeKeepsExplicitProject(t *testing.T) {
	var gotScopeDescriptor string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/myorg/_apis/projects/MyProj":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "proj-guid-1"})
		case r.URL.Path == "/myorg/_apis/Graph/Descriptors/proj-guid-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"value": "scope-descriptor-1"})
		case r.URL.Path == "/myorg/_apis/Graph/Groups":
			gotScopeDescriptor = r.URL.Query().Get("scopeDescriptor")
			_ = json.NewEncoder(w).Encode(map[string]any{"count": 0, "value": []any{}})
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	securityWithTestClient(t, srv)

	cmd := securityGroupListCmd()
	cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
	cmd.Flags().Set("detect", "false")
	cmd.Flags().Set("scope", "organization")
	cmd.Flags().Set("project", "MyProj")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("query", "", "")

	if err := securityGroupListRun(context.Background(), cmd); err != nil {
		t.Fatalf("securityGroupListRun: %v", err)
	}
	if gotScopeDescriptor != "scope-descriptor-1" {
		t.Errorf("scopeDescriptor = %q, want scope-descriptor-1 (explicit --project kept for org scope)", gotScopeDescriptor)
	}
}
