package ado

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

// getCredential is a package-level var so tests can substitute a fake.
var getCredential = azure.GetCredential

// adoResourceID is the well-known first-party Azure DevOps AAD resource id
// (services.py:157-158).
const adoResourceID = "499b84ac-1321-427f-aa17-267ca6975798"

// errNoAuth is returned verbatim (services.py:81-83, suffix from
// services.py:429-430) when neither an AAD login nor a PAT is available.
const errNoAuth = "Before you can run Azure DevOps commands, you need to run the login command" +
	"(az login if using AAD/MSA identity else az devops login if using PAT token) to setup " +
	"credentials.  Please see https://aka.ms/azure-devops-cli-auth for more information."

// basicAuth builds an Azure DevOps Basic auth header from a bearer/PAT
// token. Both AAD tokens and PATs go out as HTTP Basic with an empty
// username (services.py:63,76, both BasicAuthentication with an empty first
// argument) — there is no Bearer header anywhere in this client.
func basicAuth(token string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+token))
}

// ResolveAuth returns the primary Authorization header and, when the primary is
// an AAD token and a PAT also exists, a PAT header to retry with on 401/203.
// Exported for the one caller (pipelines' artifact download/upload) that has
// to build its own requests instead of going through Client.
// Ported from services.py:54-83 (_get_credentials). AAD is tried first, but
// a PAT wins whenever AAD is unavailable — see foundation-spec.md §3.2.
func ResolveAuth(ctx context.Context, org string) (primary, fallback string, err error) {
	var aadToken string
	cred, cerr := getCredential()
	if cerr != nil {
		logger.Debug("az login is not present: %v", cerr)
	} else {
		tok, terr := cred.GetToken(ctx, policy.TokenRequestOptions{
			Scopes: []string{adoResourceID + "/.default"},
		})
		if terr != nil {
			logger.Debug("failed to acquire AAD token: %v", terr)
		} else {
			aadToken = tok.Token
		}
	}

	// services.py:69-73 tests membership (`PAT_ENV_VARIABLE_NAME in os.environ`),
	// not truthiness, so an exported-but-empty env var is still used as the
	// credential rather than falling through to the stored PAT.
	pat, patPresent := os.LookupEnv("AZURE_DEVOPS_EXT_PAT")
	if !patPresent {
		pat = GetPAT(org)
		patPresent = pat != ""
	}

	switch {
	case aadToken != "" && patPresent:
		return basicAuth(aadToken), basicAuth(pat), nil
	case aadToken != "":
		return basicAuth(aadToken), "", nil
	case patPresent:
		return basicAuth(pat), "", nil
	default:
		return "", "", errors.New(errNoAuth)
	}
}

// patOptionKey is the INI option name Python's configparser writes
// (credential_store._USERNAME, credential_store.py:158). Python lowercases
// option names on write, so reads match it case-insensitively.
const patOptionKey = "personal access token"

const patStoreFile = "personalAccessTokens"

// configDir is $AZURE_DEVOPS_EXT_CONFIG_DIR if set (config.py:20), else
// <configDir>/azuredevops where configDir is $AZURE_CONFIG_DIR if set, else
// ~/.azure (const.py:11-13, azure.cli.core._environment.get_config_dir).
func configDir() string {
	if d := os.Getenv("AZURE_DEVOPS_EXT_CONFIG_DIR"); d != "" {
		return d
	}
	base := os.Getenv("AZURE_CONFIG_DIR")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".azure")
	}
	return filepath.Join(base, "azuredevops")
}

// patKey returns "azdevops-cli: default" for an empty org, else
// "azdevops-cli:" + the normalised org URL (_credentials.py:74-87).
func patKey(org string) string {
	if org == "" {
		return "azdevops-cli: default"
	}
	return "azdevops-cli:" + normalizeOrgForKey(org)
}

// normalizeOrgForKey lowercases the scheme and host, and — only when the raw
// URL does NOT contain "visualstudio.com" — appends "/" + the lowercased
// first path segment (the org name in a dev.azure.com URL). The
// visualstudio.com check is against the whole raw URL, not just the parsed
// host, matching _credentials.py:84 (`'visualstudio.com' not in url.lower()`).
func normalizeOrgForKey(org string) string {
	u, err := url.Parse(org)
	if err != nil {
		return strings.ToLower(org)
	}
	base := strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
	if strings.Contains(strings.ToLower(org), "visualstudio.com") {
		return base
	}
	seg := strings.Trim(u.Path, "/")
	if i := strings.Index(seg, "/"); i >= 0 {
		seg = seg[:i]
	}
	if seg == "" {
		return base
	}
	return base + "/" + strings.ToLower(seg)
}

// loadPATStore reads <configDir>/personalAccessTokens into key -> PAT. The
// file has exactly one INI option per section (patOptionKey), so there is no
// need to model the full INI shape — just the value we actually store.
func loadPATStore() (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(configDir(), patStoreFile))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("failed to read PAT store: %w", err)
	}

	store := map[string]string{}
	for section, opts := range parseINI(string(data)) {
		if pat := opts[patOptionKey]; pat != "" {
			store[section] = pat
		}
	}
	return store, nil
}

func savePATStore(store map[string]string) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	keys := make([]string, 0, len(store))
	for key := range store {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "[%s]\n", key)
		fmt.Fprintf(&b, "%s = %s\n", patOptionKey, store[key])
	}

	if err := os.WriteFile(filepath.Join(dir, patStoreFile), []byte(b.String()), 0600); err != nil {
		return fmt.Errorf("failed to write PAT store: %w", err)
	}
	return nil
}

// GetPAT returns the PAT stored for org, falling back to the default key,
// or "" if none is stored.
func GetPAT(org string) string {
	store, err := loadPATStore()
	if err != nil {
		return ""
	}
	if pat := store[patKey(org)]; pat != "" {
		return pat
	}
	if org != "" {
		return store[patKey("")]
	}
	return ""
}

// SetPAT stores pat under org's key (org may be "" for the default key).
func SetPAT(org, pat string) error {
	store, err := loadPATStore()
	if err != nil {
		return err
	}
	store[patKey(org)] = pat
	return savePATStore(store)
}

// ClearPAT removes the PAT for org. org == "" removes the whole store,
// mirroring `az devops logout` with no --organization (_credentials.py:50-56
// clearing every entry then deleting the file — our store is one file, so
// deleting it does the same thing in one step).
func ClearPAT(org string) error {
	store, err := loadPATStore()
	if err != nil {
		return err
	}

	if org == "" {
		if len(store) == 0 {
			return errors.New("No credentials were found.")
		}
		_ = os.Remove(filepath.Join(configDir(), patStoreFile))
		return nil
	}

	key := patKey(org)
	if _, ok := store[key]; !ok {
		return errors.New("No credentials were found.")
	}
	delete(store, key)
	return savePATStore(store)
}
