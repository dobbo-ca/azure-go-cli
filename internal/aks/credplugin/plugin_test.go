package credplugin

import "testing"

func TestDetermineAPIVersion(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		want    string
		wantErr bool
	}{
		{name: "empty env defaults to v1beta1", env: "", want: APIVersionV1Beta1},
		{name: "explicit v1beta1", env: `{"apiVersion":"client.authentication.k8s.io/v1beta1","kind":"ExecCredential"}`, want: APIVersionV1Beta1},
		{name: "explicit v1", env: `{"apiVersion":"client.authentication.k8s.io/v1","kind":"ExecCredential"}`, want: APIVersionV1},
		{name: "envelope without apiVersion defaults to v1beta1", env: `{"kind":"ExecCredential"}`, want: APIVersionV1Beta1},
		{name: "unknown apiVersion errors", env: `{"apiVersion":"bogus/v9"}`, wantErr: true},
		{name: "malformed json errors", env: `{not json`, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetermineAPIVersion(tc.env)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (result=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
