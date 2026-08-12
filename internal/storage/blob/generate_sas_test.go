package blob

import (
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
