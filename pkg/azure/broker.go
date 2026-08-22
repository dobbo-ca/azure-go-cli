package azure

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/cdobbyn/azure-go-cli/pkg/azure/msalruntime"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

// AuthModeBroker signs in through the Windows WAM broker (msalruntime.dll),
// which satisfies device-based Conditional Access without a browser. Only
// reachable on Windows, and only when the DLL is already installed - we can't
// ship it. See BrokerAvailable.
const AuthModeBroker = "broker"

// brokerClientID is the Azure CLI's client ID, as used everywhere else here.
const brokerClientID = "04b07795-8ddb-461a-bbee-02f9e1bf7b46"

// ErrBrokerUnavailable reports that the WAM broker can't serve this machine,
// either because msalruntime.dll is missing or because the broker itself said
// so mid-flight. Callers test for it with errors.Is and fall back to the
// browser flow.
var ErrBrokerUnavailable = msalruntime.ErrNotAvailable

// currentBrokerAccountID is the broker account ID from this process's
// sign-in. Login authenticates before a profile exists to read it from.
var currentBrokerAccountID string

// BrokerAccountID returns the broker account ID for the active sign-in: this
// process's if it just signed in, otherwise the saved profile's.
func BrokerAccountID() string {
	if currentBrokerAccountID != "" {
		return currentBrokerAccountID
	}
	profile, err := config.Load()
	if err != nil {
		return ""
	}
	return profile.BrokerAccountID
}

// BrokerAvailable reports whether the WAM broker can be used. Failure is the
// normal case (non-Windows, or Windows without msalruntime.dll), so it's
// logged at debug level and the caller silently falls back to the browser flow.
func BrokerAvailable() bool {
	if err := msalruntime.Startup(); err != nil {
		logger.Debug("WAM broker unavailable, using browser sign-in: %v", err)
		return false
	}
	return true
}

// BrokerCredential acquires tokens through the WAM broker. Silent acquisition
// is tried first; interactive credentials fall back to the broker's sign-in UI
// when the broker reports that interaction is required.
type BrokerCredential struct {
	authority   string
	interactive bool
	accountID   string
}

// NewBrokerCredential creates a broker credential for the given tenant
// ("organizations" for the initial sign-in). Only the credential used for the
// initial sign-in is interactive; tenant-scoped credentials are silent, so
// discovering N tenants can't pop N sign-in windows.
func NewBrokerCredential(tenantID string, interactive bool) (*BrokerCredential, error) {
	if err := msalruntime.Startup(); err != nil {
		return nil, err
	}
	return &BrokerCredential{
		authority:   "https://login.microsoftonline.com/" + tenantID,
		interactive: interactive,
		accountID:   BrokerAccountID(),
	}, nil
}

// Authenticate performs the initial sign-in and returns an authentication
// record. Discovery calls this on the base credential; the broker account ID
// it establishes is what every later silent acquisition keys off.
func (b *BrokerCredential) Authenticate(ctx context.Context) (azidentity.AuthenticationRecord, error) {
	res, err := b.token(ctx, []string{"https://management.azure.com/.default"})
	if err != nil {
		return azidentity.AuthenticationRecord{}, err
	}
	return azidentity.AuthenticationRecord{
		Authority:     "login.microsoftonline.com",
		HomeAccountID: res.HomeAccountID,
		ClientID:      brokerClientID,
		Version:       "1.0",
	}, nil
}

// GetToken implements azcore.TokenCredential.
func (b *BrokerCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	scopes := opts.Scopes
	if len(scopes) == 0 {
		scopes = []string{"https://management.azure.com/.default"}
	}
	res, err := b.token(ctx, scopes)
	if err != nil {
		return azcore.AccessToken{}, err
	}
	return azcore.AccessToken{Token: res.AccessToken, ExpiresOn: res.ExpiresOn}, nil
}

func (b *BrokerCredential) token(ctx context.Context, scopes []string) (*msalruntime.AuthResult, error) {
	if b.accountID != "" {
		res, err := b.silent(ctx, scopes)
		if err == nil {
			return res, nil
		}
		if !b.interactive || !interactionRequired(err) {
			return nil, err
		}
		logger.Debug("Broker requires interaction for %s: %v", b.authority, err)
	} else if !b.interactive {
		return nil, fmt.Errorf("no broker account for silent token acquisition. Please run 'az login'")
	}

	res, err := msalruntime.SignInInteractively(ctx, b.authority, brokerClientID, scopes, 0)
	if err != nil {
		return nil, err
	}
	if res.AccountID != "" {
		b.accountID = res.AccountID
		currentBrokerAccountID = res.AccountID
	}
	return res, nil
}

func (b *BrokerCredential) silent(ctx context.Context, scopes []string) (*msalruntime.AuthResult, error) {
	account, err := msalruntime.ReadAccountByID(ctx, b.accountID)
	if err != nil {
		return nil, err
	}
	defer account.Close()
	return msalruntime.AcquireTokenSilently(ctx, account, b.authority, brokerClientID, scopes)
}

// interactionRequired reports whether the broker failed for a reason that
// signing in interactively can fix.
func interactionRequired(err error) bool {
	var brokerErr *msalruntime.Error
	if !errors.As(err, &brokerErr) {
		return false
	}
	switch brokerErr.Status {
	case msalruntime.StatusInteractionRequired, msalruntime.StatusAccountNotFound, msalruntime.StatusAccountUnusable:
		return true
	}
	return false
}
