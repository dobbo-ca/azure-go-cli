package blob

import "testing"

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
