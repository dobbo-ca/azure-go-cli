package ado

import (
	"net/url"
	"os/exec"
	"strings"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

// remoteInfo is what a recognised Azure DevOps remote yields.
type remoteInfo struct{ Org, Project, Repo string }

// detectFromGitRemote runs `git remote -v` in the process working directory
// and returns the first Azure DevOps remote it recognises, or ok=false. This
// is the only external binary the port shells out to. It is a package-level
// var so tests can substitute a fake, the same seam as getCredential.
var detectFromGitRemote = func() (remoteInfo, bool) {
	out, err := exec.Command("git", "remote", "-v").Output()
	if err != nil {
		logger.Debug("git remote -v failed: %v", err)
		return remoteInfo{}, false
	}
	return selectRemote(string(out))
}

// selectRemote picks a candidate remote from `git remote -v` output
// (git.py:59-69,105,115-118): prefer origin's push URL if it is a
// recognised Azure DevOps remote, otherwise the first *(push) entry that is.
// Fetch-only entries are ignored.
func selectRemote(gitRemoteV string) (remoteInfo, bool) {
	push := map[string]string{}
	var order []string

	for _, line := range strings.Split(gitRemoteV, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[2] != "(push)" {
			continue
		}
		name, remoteURL := fields[0], fields[1]
		if _, seen := push[name]; !seen {
			order = append(order, name)
		}
		push[name] = remoteURL
	}

	if u, ok := push["origin"]; ok {
		if info, ok := parseRemoteURL(u); ok {
			return info, true
		}
	}
	for _, name := range order {
		if name == "origin" {
			continue
		}
		if info, ok := parseRemoteURL(push[name]); ok {
			return info, true
		}
	}

	return remoteInfo{}, false
}

// parseRemoteURL recognises an Azure DevOps git remote URL and extracts
// (org, project, repo) directly from the URL. Deliberate deviation from
// Python: azext_devops calls the Azure DevOps API
// (GitClient.get_vsts_info_by_remote_url, vsts_git_url_info.py:100) and
// returns project/repo as GUIDs, with an on-disk cache. Azure DevOps routes
// accept a name or id interchangeably for both, so this port parses the URL
// locally and returns names instead — see foundation-spec.md §4.6.
func parseRemoteURL(remote string) (remoteInfo, bool) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return remoteInfo{}, false
	}

	scheme, host, path, user, ok := splitRemoteURL(remote)
	if !ok || host == "" {
		return remoteInfo{}, false
	}
	lhost := strings.ToLower(host)

	switch {
	case lhost == "dev.azure.com":
		if scheme == "ssh" {
			return remoteInfo{}, false
		}
		return parseHTTPGitRemote(path, "https://dev.azure.com", false)

	case lhost == "ssh.dev.azure.com":
		if user == "" { // on-prem-style ssh, unsupported (vsts_git_url_info.py:114-118)
			return remoteInfo{}, false
		}
		org, project, repo, ok := parseV3Path(path)
		if !ok {
			return remoteInfo{}, false
		}
		return remoteInfo{Org: "https://dev.azure.com/" + org, Project: project, Repo: repo}, true

	case lhost == "vs-ssh.visualstudio.com":
		if user == "" {
			return remoteInfo{}, false
		}
		_, project, repo, ok := parseV3Path(path)
		if !ok {
			return remoteInfo{}, false
		}
		return remoteInfo{Org: "https://" + user + ".visualstudio.com", Project: project, Repo: repo}, true

	case strings.HasSuffix(lhost, ".visualstudio.com"):
		if scheme == "ssh" {
			// Only the two special hosts above carry ssh for visualstudio.com.
			return remoteInfo{}, false
		}
		return parseHTTPGitRemote(path, "https://"+host, true)
	}

	return remoteInfo{}, false
}

// splitRemoteURL parses remote into scheme, host, path and ssh userinfo (if
// any). It falls back to splitSCPLike for git's scp-like shorthand
// ("user@host:path"), which net/url cannot parse once given a scheme: the
// text after the colon ("v3/org/project/repo") is not a valid port. Python's
// looser urlparse tolerates it (uri.py:14-19); see splitSCPLike for how this
// port replicates that behaviour without it.
func splitRemoteURL(remote string) (scheme, host, path, user string, ok bool) {
	raw := remote
	if !strings.Contains(remote, "://") {
		raw = "ssh://" + remote
	}

	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		if u.User != nil {
			user = u.User.Username()
		}
		return u.Scheme, u.Hostname(), u.Path, user, true
	}

	h, p, u, ok := splitSCPLike(remote)
	if !ok {
		return "", "", "", "", false
	}
	return "ssh", h, p, u, true
}

// splitSCPLike parses "[user@]host:path", replicating what Python's
// urlparse("ssh://"+url) does for it (vsts_git_url_info.py:64-90): the text
// up to the first "/" becomes the netloc, and anything after the last ":" in
// that netloc — here, the literal "v3" — is dropped as a bogus port. That is
// exactly how the real ssh clone URLs Azure Repos hands out
// ("host:v3/org/project/repo") fold down to a plain "/org/project/repo" path.
func splitSCPLike(remote string) (host, path, user string, ok bool) {
	rest := strings.TrimPrefix(remote, "ssh://")

	if i := strings.Index(rest, "@"); i >= 0 {
		user = rest[:i]
		rest = rest[i+1:]
	}

	slash := strings.Index(rest, "/")
	if slash < 0 {
		return "", "", "", false
	}
	netloc := rest[:slash]
	path = rest[slash:]
	if c := strings.Index(netloc, ":"); c >= 0 {
		netloc = netloc[:c]
	}
	if netloc == "" {
		return "", "", "", false
	}
	return netloc, path, user, true
}

// parseHTTPGitRemote extracts (org, project, repo) from an https-style
// Azure Repos remote path containing a /_git/ or /_ssh/ marker.
func parseHTTPGitRemote(path, orgBase string, orgFromHost bool) (remoteInfo, bool) {
	if !strings.Contains(path, "/_git/") && !strings.Contains(path, "/_ssh/") {
		return remoteInfo{}, false
	}
	path = strings.Replace(path, "/_ssh/", "/_git/", 1)

	org, project, repo, ok := splitGitPath(path, orgFromHost)
	if !ok {
		return remoteInfo{}, false
	}
	if orgFromHost {
		return remoteInfo{Org: orgBase, Project: project, Repo: repo}, true
	}
	return remoteInfo{Org: orgBase + "/" + org, Project: project, Repo: repo}, true
}

// splitGitPath extracts (org, project, repo) from a path shaped like
// "{org}/{project}/_git/{repo}" (org from the path, orgFromHost false) or
// "[DefaultCollection/]{project}/_git/{repo}" (org from the host, so
// DefaultCollection — the legacy on-prem collection segment some
// visualstudio.com remotes still carry — is stripped if present).
func splitGitPath(path string, orgFromHost bool) (org, project, repo string, ok bool) {
	segs := pathSegments(path)

	idx := -1
	for i, s := range segs {
		if s == "_git" {
			idx = i
			break
		}
	}
	if idx < 0 || idx+2 > len(segs) {
		return "", "", "", false
	}
	repo = segs[idx+1]

	head := segs[:idx]
	if orgFromHost {
		if len(head) > 0 && head[0] == "DefaultCollection" {
			head = head[1:]
		}
	} else {
		if len(head) < 1 {
			return "", "", "", false
		}
		org = head[0]
		head = head[1:]
	}

	if len(head) != 1 {
		return "", "", "", false
	}
	project = head[0]
	return org, project, repo, true
}

// parseV3Path extracts (org, project, repo) from the new-format Azure Repos
// ssh path "/org/project/repo" left over after splitSCPLike folds "v3" away
// (vsts_git_url_info.py:75-82).
func parseV3Path(path string) (org, project, repo string, ok bool) {
	segs := pathSegments(path)
	if len(segs) != 3 {
		return "", "", "", false
	}
	return segs[0], segs[1], segs[2], true
}

// pathSegments splits path on "/" into non-empty, URL-decoded segments
// (project/repo names routinely contain "%20").
func pathSegments(path string) []string {
	var segs []string
	for _, s := range strings.Split(strings.Trim(path, "/"), "/") {
		if s == "" {
			continue
		}
		if d, err := url.PathUnescape(s); err == nil {
			s = d
		}
		segs = append(segs, s)
	}
	return segs
}
