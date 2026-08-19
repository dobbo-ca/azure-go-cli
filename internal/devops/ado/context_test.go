package ado

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		name    string
		remote  string
		wantOrg string
		wantP   string
		wantR   string
		wantOK  bool
	}{
		{
			name:    "dev.azure.com https",
			remote:  "https://dev.azure.com/myorg/myproj/_git/myrepo",
			wantOrg: "https://dev.azure.com/myorg",
			wantP:   "myproj",
			wantR:   "myrepo",
			wantOK:  true,
		},
		{
			name:    "visualstudio.com https",
			remote:  "https://myorg.visualstudio.com/myproj/_git/myrepo",
			wantOrg: "https://myorg.visualstudio.com",
			wantP:   "myproj",
			wantR:   "myrepo",
			wantOK:  true,
		},
		{
			name:    "visualstudio.com https with DefaultCollection",
			remote:  "https://myorg.visualstudio.com/DefaultCollection/myproj/_git/myrepo",
			wantOrg: "https://myorg.visualstudio.com",
			wantP:   "myproj",
			wantR:   "myrepo",
			wantOK:  true,
		},
		{
			name:    "ssh.dev.azure.com v3",
			remote:  "git@ssh.dev.azure.com:v3/myorg/myproj/myrepo",
			wantOrg: "https://dev.azure.com/myorg",
			wantP:   "myproj",
			wantR:   "myrepo",
			wantOK:  true,
		},
		{
			name:    "vs-ssh.visualstudio.com v3",
			remote:  "myorg@vs-ssh.visualstudio.com:v3/myorg/myproj/myrepo",
			wantOrg: "https://myorg.visualstudio.com",
			wantP:   "myproj",
			wantR:   "myrepo",
			wantOK:  true,
		},
		{
			name:    "dev.azure.com https with _ssh marker",
			remote:  "https://dev.azure.com/myorg/myproj/_ssh/myrepo",
			wantOrg: "https://dev.azure.com/myorg",
			wantP:   "myproj",
			wantR:   "myrepo",
			wantOK:  true,
		},
		{
			name:    "project name with %20",
			remote:  "https://dev.azure.com/myorg/My%20Project/_git/myrepo",
			wantOrg: "https://dev.azure.com/myorg",
			wantP:   "My Project",
			wantR:   "myrepo",
			wantOK:  true,
		},
		{
			name:   "on-prem ssh unsupported",
			remote: "user@onprem:22/tfs/DefaultCollection/proj/_git/repo",
			wantOK: false,
		},
		{
			name:   "non-ADO url",
			remote: "https://github.com/a/b.git",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := parseRemoteURL(tt.remote)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (info=%+v)", ok, tt.wantOK, info)
			}
			if !tt.wantOK {
				return
			}
			if info.Org != tt.wantOrg || info.Project != tt.wantP || info.Repo != tt.wantR {
				t.Errorf("got %+v, want {Org:%q Project:%q Repo:%q}", info, tt.wantOrg, tt.wantP, tt.wantR)
			}
		})
	}
}

func TestSelectRemote(t *testing.T) {
	t.Run("origin preferred", func(t *testing.T) {
		fixture := "origin\thttps://dev.azure.com/myorg/myproj/_git/myrepo (fetch)\n" +
			"origin\thttps://dev.azure.com/myorg/myproj/_git/myrepo (push)\n" +
			"upstream\thttps://dev.azure.com/other/otherproj/_git/otherrepo (push)\n"

		info, ok := selectRemote(fixture)
		if !ok {
			t.Fatal("selectRemote: want ok")
		}
		if info.Project != "myproj" {
			t.Errorf("Project = %q, want %q (origin should win)", info.Project, "myproj")
		}
	})

	t.Run("first ADO push wins when origin is GitHub", func(t *testing.T) {
		fixture := "origin\thttps://github.com/a/b.git (fetch)\n" +
			"origin\thttps://github.com/a/b.git (push)\n" +
			"ado\thttps://dev.azure.com/myorg/myproj/_git/myrepo (fetch)\n" +
			"ado\thttps://dev.azure.com/myorg/myproj/_git/myrepo (push)\n"

		info, ok := selectRemote(fixture)
		if !ok {
			t.Fatal("selectRemote: want ok")
		}
		if info.Project != "myproj" {
			t.Errorf("Project = %q, want %q", info.Project, "myproj")
		}
	})

	t.Run("fetch-only entries ignored", func(t *testing.T) {
		fixture := "origin\thttps://dev.azure.com/myorg/myproj/_git/myrepo (fetch)\n"

		if _, ok := selectRemote(fixture); ok {
			t.Fatal("selectRemote: want !ok, fetch-only entries must be ignored")
		}
	})
}

func TestResolvePrecedence(t *testing.T) {
	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{Use: "test"}
		AddOrgFlags(cmd)
		AddProjectFlag(cmd)
		AddRepoFlag(cmd)
		return cmd
	}

	t.Run("explicit org suppresses detect and configured project default", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		if err := SetConfigDefaults("https://dev.azure.com/configorg", "configproj"); err != nil {
			t.Fatalf("SetConfigDefaults: %v", err)
		}
		orig := detectFromGitRemote
		detectFromGitRemote = func() (remoteInfo, bool) {
			return remoteInfo{Org: "https://dev.azure.com/detected", Project: "detectedproj", Repo: "detectedrepo"}, true
		}
		t.Cleanup(func() { detectFromGitRemote = orig })

		cmd := newCmd()
		cmd.Flags().Set("organization", "https://dev.azure.com/explicitorg")

		ctx, err := Resolve(cmd)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if ctx.Org != "https://dev.azure.com/explicitorg" {
			t.Errorf("Org = %q, want explicit org", ctx.Org)
		}
		if ctx.Project != "" {
			t.Errorf("Project = %q, want empty (configured default must be suppressed)", ctx.Project)
		}

		if _, err := ResolveProject(cmd); err == nil || err.Error() != errNoProject {
			t.Errorf("ResolveProject err = %v, want errNoProject", err)
		}
	})

	t.Run("detect beats config", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		if err := SetConfigDefaults("https://dev.azure.com/configorg", "configproj"); err != nil {
			t.Fatalf("SetConfigDefaults: %v", err)
		}
		orig := detectFromGitRemote
		detectFromGitRemote = func() (remoteInfo, bool) {
			return remoteInfo{Org: "https://dev.azure.com/detected", Project: "detectedproj", Repo: "detectedrepo"}, true
		}
		t.Cleanup(func() { detectFromGitRemote = orig })

		ctx, err := ResolveRepo(newCmd())
		if err != nil {
			t.Fatalf("ResolveRepo: %v", err)
		}
		if ctx.Org != "https://dev.azure.com/detected" || ctx.Project != "detectedproj" || ctx.Repo != "detectedrepo" {
			t.Errorf("got %+v, want detected values", ctx)
		}
	})

	t.Run("config used when detect is off", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		if err := SetConfigDefaults("https://dev.azure.com/configorg", "configproj"); err != nil {
			t.Fatalf("SetConfigDefaults: %v", err)
		}
		orig := detectFromGitRemote
		detectFromGitRemote = func() (remoteInfo, bool) {
			t.Fatal("detectFromGitRemote must not be consulted when --detect=false")
			return remoteInfo{}, false
		}
		t.Cleanup(func() { detectFromGitRemote = orig })

		cmd := newCmd()
		cmd.Flags().Set("detect", "false")

		ctx, err := ResolveProject(cmd)
		if err != nil {
			t.Fatalf("ResolveProject: %v", err)
		}
		if ctx.Org != "https://dev.azure.com/configorg" || ctx.Project != "configproj" {
			t.Errorf("got %+v, want configured defaults", ctx)
		}
	})

	t.Run("missing org", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		orig := detectFromGitRemote
		detectFromGitRemote = func() (remoteInfo, bool) { return remoteInfo{}, false }
		t.Cleanup(func() { detectFromGitRemote = orig })

		if _, err := Resolve(newCmd()); err == nil || err.Error() != errNoOrg {
			t.Errorf("err = %v, want errNoOrg", err)
		}
	})

	t.Run("bare --detect is accepted and means true", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		orig := detectFromGitRemote
		detectFromGitRemote = func() (remoteInfo, bool) {
			return remoteInfo{Org: "https://dev.azure.com/detected"}, true
		}
		t.Cleanup(func() { detectFromGitRemote = orig })

		cmd := newCmd()
		if err := cmd.ParseFlags([]string{"--detect"}); err != nil {
			t.Fatalf("ParseFlags(--detect): %v", err)
		}

		ctx, err := Resolve(cmd)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if ctx.Org != "https://dev.azure.com/detected" {
			t.Errorf("Org = %q, want detected org (bare --detect must mean true)", ctx.Org)
		}
	})

	t.Run("invalid --detect value errors instead of silently detecting", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		orig := detectFromGitRemote
		detectFromGitRemote = func() (remoteInfo, bool) {
			t.Fatal("detectFromGitRemote must not be consulted for an invalid --detect value")
			return remoteInfo{}, false
		}
		t.Cleanup(func() { detectFromGitRemote = orig })

		cmd := newCmd()
		cmd.Flags().Set("detect", "no")

		if _, err := Resolve(cmd); err == nil {
			t.Fatal("Resolve: want error for --detect=no, got nil")
		}
	})

	t.Run("missing project reported before on-prem org error", func(t *testing.T) {
		// services.py:360-361 resolves/validates project before validating
		// org at services.py:366-370, so a missing --project wins even when
		// the org is also on-prem.
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		orig := detectFromGitRemote
		detectFromGitRemote = func() (remoteInfo, bool) { return remoteInfo{}, false }
		t.Cleanup(func() { detectFromGitRemote = orig })

		cmd := newCmd()
		cmd.Flags().Set("organization", "https://tfs.contoso.com/tfs/DefaultCollection")

		if _, err := ResolveProject(cmd); err == nil || err.Error() != errNoProject {
			t.Errorf("err = %v, want errNoProject", err)
		}
	})

	t.Run("missing repo via ResolveRepo", func(t *testing.T) {
		t.Setenv("AZURE_DEVOPS_EXT_CONFIG_DIR", t.TempDir())
		orig := detectFromGitRemote
		detectFromGitRemote = func() (remoteInfo, bool) { return remoteInfo{}, false }
		t.Cleanup(func() { detectFromGitRemote = orig })

		cmd := newCmd()
		cmd.Flags().Set("organization", "https://dev.azure.com/myorg")
		cmd.Flags().Set("project", "myproj")

		if _, err := ResolveRepo(cmd); err == nil || err.Error() != errNoRepo {
			t.Errorf("err = %v, want errNoRepo", err)
		}
	})
}

func TestValidateOrg(t *testing.T) {
	tests := []struct {
		name    string
		org     string
		want    string
		wantErr string
	}{
		{name: "dev.azure.com", org: "https://dev.azure.com/x", want: "https://dev.azure.com/x"},
		{name: "visualstudio.com trailing slash stripped", org: "https://x.visualstudio.com/", want: "https://x.visualstudio.com"},
		{name: "on-prem rejected", org: "https://tfs.contoso.com/tfs/DefaultCollection", wantErr: errOnPrem},
		{name: "not a url", org: "not-a-url", wantErr: errNoOrg},
		// services.py:446 checks startswith("https://dev.azure.com/") against
		// the RAW value — a bare org URL with just its own trailing slash
		// (no org name after it) still satisfies that prefix.
		{name: "dev.azure.com bare trailing slash accepted", org: "https://dev.azure.com/", want: "https://dev.azure.com"},
		// services.py:447 rstrips ALL trailing slashes before the endswith
		// check, not just one.
		{name: "visualstudio.com double trailing slash accepted", org: "https://myorg.visualstudio.com//", want: "https://myorg.visualstudio.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateOrg(tt.org)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateOrg: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if strings.HasSuffix(got, "/") {
				t.Errorf("got %q, trailing slash not stripped", got)
			}
		})
	}
}
