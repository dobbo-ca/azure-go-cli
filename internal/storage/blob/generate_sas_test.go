package blob

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

// TestFullURIPathDecodesToSignedName locks the invariant that makes --full-uri
// correct: the SAS signature is computed over the raw blob name (it is assigned
// verbatim to sas.BlobSignatureValues.BlobName), so the URL's path MUST decode
// back to exactly that string. If it does not, the URL addresses a different
// blob than the one signed and the service rejects it with an opaque
// AuthenticationFailed.
//
// This is why '%' is not in pathSafe. Marking it safe passed a literal '%'
// straight through, which either produced an unparseable URL ("a%b") or
// silently retargeted the request ("my%20file.txt" signed, but addressing
// "my file.txt").
func TestFullURIPathDecodesToSignedName(t *testing.T) {
	names := []string{
		"plain.txt",
		"my blob.txt",
		"a%b",           // bare percent: invalid in a URL unless escaped
		"my%20file.txt", // looks pre-encoded, but is a literal name
		"100%.txt",
		"logs/2026/app.log", // virtual directory: slashes must survive
		"a+b&c=d",
		"café.txt", // multi-byte UTF-8
		"a(b)$c=d", // sub-delims: azure-cli escapes these
		"o'brien.txt",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			raw := fullURI("https://acct.blob.core.windows.net", "c", name, "", "sig=abc")
			u, err := url.Parse(raw)
			if err != nil {
				t.Fatalf("fullURI produced an unparseable URL %q: %v", raw, err)
			}
			got, ok := strings.CutPrefix(u.Path, "/c/")
			if !ok {
				t.Fatalf("path %q does not start with /c/", u.Path)
			}
			if got != name {
				t.Errorf("path decoded to %q, want %q (signature was computed over %q)", got, name, name)
			}
		})
	}
}

func TestFullURI(t *testing.T) {
	cases := []struct {
		name          string
		endpoint      string
		containerName string
		blobName      string
		snapshot      string
		token         string
		want          string
	}{
		{
			name:          "space in blob name",
			endpoint:      "https://myaccount.blob.core.windows.net",
			containerName: "mycontainer",
			blobName:      "my blob.txt",
			token:         "se=2030-01-01&sig=abc%2Fdef",
			want:          "https://myaccount.blob.core.windows.net/mycontainer/my%20blob.txt?se=2030-01-01&sig=abc%2Fdef",
		},
		{
			name:          "nested blob name keeps slashes",
			endpoint:      "https://myaccount.blob.core.windows.net",
			containerName: "mycontainer",
			blobName:      "logs/2026/app.log",
			token:         "se=2030-01-01&sig=abc",
			want:          "https://myaccount.blob.core.windows.net/mycontainer/logs/2026/app.log?se=2030-01-01&sig=abc",
		},
		{
			name:          "literal percent in blob name is escaped, not passed through",
			endpoint:      "https://myaccount.blob.core.windows.net",
			containerName: "mycontainer",
			blobName:      "a%b",
			token:         "sig=abc",
			want:          "https://myaccount.blob.core.windows.net/mycontainer/a%25b?sig=abc",
		},
		{
			name:          "snapshot is threaded ahead of the token",
			endpoint:      "https://myaccount.blob.core.windows.net",
			containerName: "c",
			blobName:      "b",
			snapshot:      "2026-01-01T00:00:00.0000000Z",
			token:         "sp=r&sr=bs&sig=abc",
			want:          "https://myaccount.blob.core.windows.net/c/b?snapshot=2026-01-01T00%3A00%3A00.0000000Z&sp=r&sr=bs&sig=abc",
		},
		{
			name:          "sovereign cloud endpoint from --blob-url is preserved",
			endpoint:      "https://chinaacct.blob.core.chinacloudapi.cn",
			containerName: "c",
			blobName:      "b",
			token:         "sig=xyz",
			want:          "https://chinaacct.blob.core.chinacloudapi.cn/c/b?sig=xyz",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fullURI(tc.endpoint, tc.containerName, tc.blobName, tc.snapshot, tc.token)
			if got != tc.want {
				t.Errorf("fullURI() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseBlobURL(t *testing.T) {
	cases := []struct {
		name                                     string
		url                                      string
		account, container, blob, snap, endpoint string
	}{
		{
			name:      "public endpoint: account is the first host label",
			url:       "https://myaccount.blob.core.windows.net/c/b.txt",
			account:   "myaccount",
			container: "c",
			blob:      "b.txt",
			endpoint:  "https://myaccount.blob.core.windows.net",
		},
		{
			// Azurite and IP-addressed private endpoints. Reading the account
			// from the host here yields "127", which signs /blob/127/... and
			// drops the account from --full-uri. Verified against a live
			// Azurite: wrong gives 400/403, right gives 200.
			name:      "IP endpoint: account is the first path segment",
			url:       "http://127.0.0.1:10000/devstoreaccount1/sastest/hello.txt",
			account:   "devstoreaccount1",
			container: "sastest",
			blob:      "hello.txt",
			endpoint:  "http://127.0.0.1:10000/devstoreaccount1",
		},
		{
			name:      "virtual directory blob name survives",
			url:       "https://myaccount.blob.core.windows.net/c/logs/2026/app.log",
			account:   "myaccount",
			container: "c",
			blob:      "logs/2026/app.log",
			endpoint:  "https://myaccount.blob.core.windows.net",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseBlobURL(c.url)
			if err != nil {
				t.Fatalf("parseBlobURL(%q): %v", c.url, err)
			}
			if got.accountName != c.account {
				t.Errorf("accountName = %q, want %q", got.accountName, c.account)
			}
			if got.containerName != c.container {
				t.Errorf("containerName = %q, want %q", got.containerName, c.container)
			}
			if got.blobName != c.blob {
				t.Errorf("blobName = %q, want %q", got.blobName, c.blob)
			}
			if got.endpoint != c.endpoint {
				t.Errorf("endpoint = %q, want %q", got.endpoint, c.endpoint)
			}
		})
	}
}

// TestUserDelegationTidFlag locks the --user-delegation-tid flag surface
// against azure-cli's dev-branch _params.py:1017 (storage blob generate-sas):
// c.argument('user_delegation_tid', ..., help='The delegated user tenant id
// in Azure AD. This parameter can only be specified when using OAuth.').
func TestUserDelegationTidFlag(t *testing.T) {
	cmd := NewGenerateSASCommand()
	f := cmd.Flags().Lookup("user-delegation-tid")
	if f == nil {
		t.Fatal("--user-delegation-tid flag not registered")
	}
	if f.Value.Type() != "string" {
		t.Errorf("flag type = %q, want %q", f.Value.Type(), "string")
	}
	if f.DefValue != "" {
		t.Errorf("flag default = %q, want empty", f.DefValue)
	}
	want := "The delegated user tenant id in Azure AD. This parameter can only be specified when using OAuth."
	if f.Usage != want {
		t.Errorf("flag usage = %q, want %q", f.Usage, want)
	}
}

// TestUserDelegationTidRequiresOid locks the validation order: --user-delegation-tid
// requires --user-delegation-oid, which in turn requires --as-user. Validation
// runs before any credential lookup, so this hits no network.
func TestUserDelegationTidRequiresOid(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "tid only",
			args: []string{"--name=b", "--container-name=c", "--permissions=r", "--expiry=2026-01-02T00:00Z", "--user-delegation-tid=11111111-2222-3333-4444-555555555555"},
			want: "incorrect usage: need to specify '--user-delegation-oid' when '--user-delegation-tid' is provided",
		},
		{
			name: "tid and oid, no as-user",
			args: []string{"--name=b", "--container-name=c", "--permissions=r", "--expiry=2026-01-02T00:00Z", "--user-delegation-oid=22222222-3333-4444-5555-666666666666", "--user-delegation-tid=11111111-2222-3333-4444-555555555555"},
			want: "incorrect usage: need to specify '--as-user' when '--user-delegation-oid' is provided",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := NewGenerateSASCommand()
			if err := cmd.ParseFlags(c.args); err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}
			err := runGenerateSAS(context.Background(), cmd)
			if err == nil {
				t.Fatal("runGenerateSAS: got nil error, want validation error")
			}
			if err.Error() != c.want {
				t.Errorf("error = %q, want %q", err.Error(), c.want)
			}
		})
	}
}
