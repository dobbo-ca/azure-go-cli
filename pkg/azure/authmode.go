package azure

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

// AuthModeAzureCLI borrows tokens from the Python Azure CLI (az), which uses
// the WAM broker on Windows and therefore satisfies device-based Conditional
// Access. See BaseCredential and TenantCredential.
const AuthModeAzureCLI = "azure-cli"

// AuthModeDefault explicitly overrides to the default MSAL interactive flow.
// Distinct from the unset zero value so a plain `az login` can't fall through
// to a stale saved profile's azure-cli mode (e.g. if a previous login's
// profile delete failed).
const AuthModeDefault = "default"

// currentAuthMode overrides the mode read from the saved profile. Login sets
// this via SetAuthMode before a profile exists on disk to reflect --use-azure-cli.
var currentAuthMode string

// SetAuthMode overrides the auth mode used by BaseCredential and
// TenantCredential. Used by login, which authenticates before a profile
// exists to load the mode from. Pass AuthModeDefault, not "", to force the
// default flow - "" means "no override, fall back to the saved profile".
func SetAuthMode(mode string) {
	currentAuthMode = mode
}

// AuthMode returns the active auth mode: the login override if set, otherwise
// whatever is saved in the profile ("" if not logged in or on the default
// flow). Exported so other packages (e.g. bastion AAD SSH) can special-case
// azure-cli mode, which doesn't populate our own MSAL cache.
func AuthMode() string {
	return authMode()
}

// authMode returns the active auth mode: the login override if set, otherwise
// whatever is saved in the profile ("" if not logged in or on the default flow).
func authMode() string {
	switch currentAuthMode {
	case "":
		// No override - fall back to the saved profile below.
	case AuthModeDefault:
		return ""
	default:
		return currentAuthMode
	}
	profile, err := config.Load()
	if err != nil {
		return ""
	}
	return profile.AuthMode
}

// BaseCredential returns the credential used to discover tenants and perform
// the initial sign-in: the Azure CLI credential in azure-cli mode, otherwise
// the MSAL interactive credential.
func BaseCredential() (azcore.TokenCredential, error) {
	if authMode() == AuthModeAzureCLI {
		if err := checkAzureCLIOnPath(); err != nil {
			return nil, err
		}
		return azidentity.NewAzureCLICredential(nil)
	}
	return NewMSALInteractiveCredential()
}

// TenantCredential returns a credential scoped to a specific tenant: the
// Azure CLI credential scoped to that tenant in azure-cli mode, otherwise the
// MSAL silent credential built from the authentication record.
func TenantCredential(tenantID string, authRecord azidentity.AuthenticationRecord) (azcore.TokenCredential, error) {
	if authMode() == AuthModeAzureCLI {
		if err := checkAzureCLIOnPath(); err != nil {
			return nil, err
		}
		return azidentity.NewAzureCLICredential(&azidentity.AzureCLICredentialOptions{TenantID: tenantID})
	}
	return NewMSALSilentCredential(tenantID, authRecord)
}

// checkAzureCLIOnPath fails fast with an actionable error when azure-cli mode
// can't work: no `az` on PATH, or `az` on PATH resolving back to this very
// binary (this project also ships itself as `az`, e.g. via the Homebrew
// az-go formula). In that case AzureCLICredential shells out to ourselves
// instead of the Python Azure CLI it's meant to borrow tokens from, which
// fails confusingly deep inside token acquisition instead of here.
func checkAzureCLIOnPath() error {
	azPath, err := exec.LookPath("az")
	if err != nil {
		return fmt.Errorf("azure-cli mode requires the Azure CLI (az) on PATH: %w", err)
	}
	self, err := os.Executable()
	if err != nil {
		return nil
	}
	selfResolved, errSelf := filepath.EvalSymlinks(self)
	azResolved, errAz := filepath.EvalSymlinks(azPath)
	if errSelf == nil && errAz == nil && selfResolved == azResolved {
		return fmt.Errorf("azure-cli mode requires the Python Azure CLI, but 'az' on PATH resolves to this binary (%s) instead; install the Python azure-cli ahead of it on PATH, or run this build under a different name", azPath)
	}
	return nil
}
