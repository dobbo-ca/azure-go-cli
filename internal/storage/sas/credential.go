package sas

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

// MissingCredentialsError reproduces the message azure-cli prints at
// operations/account.py:39. It is the most common failure for this command,
// so it names every accepted way to supply credentials.
const MissingCredentialsError = `missing or invalid credentials to access the storage service. The following variations are accepted:
    (1) account name and key (--account-name and --account-key options, or
        set AZURE_STORAGE_ACCOUNT and AZURE_STORAGE_KEY environment variables)
    (2) account name (--account-name option or AZURE_STORAGE_ACCOUNT environment variable;
        this will query for a storage account key using your login credentials)
    (3) connection string (--connection-string option or
        set AZURE_STORAGE_CONNECTION_STRING environment variable)`

// Creds is a resolved storage account name and shared key.
type Creds struct {
	AccountName string
	AccountKey  string
}

// ParseConnectionString splits a storage connection string into its parts.
// It cuts on the first '=' only, so a base64 AccountKey keeps its padding.
func ParseConnectionString(cs string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(cs, ";") {
		k, v, found := strings.Cut(part, "=")
		if found {
			out[strings.TrimSpace(k)] = v
		}
	}
	return out
}

// ResolveInputs applies the flag-then-environment precedence chain from
// _validators.py:validate_client_parameters.
//
// An explicit --connection-string always wins. The AZURE_STORAGE_CONNECTION_STRING
// environment variable is consulted only when neither --connection-string nor
// --account-key was supplied, matching _validators.py:152 — so an explicit
// --account-key is never silently overridden by a stale environment variable.
//
// When a connection string is in play, its AccountName/AccountKey unconditionally
// overwrite any flag-supplied values (_validators.py:156-160), then fall through
// into the same env-backfill below — so a connection string with AccountName but
// no AccountKey still gets a chance to pick up AZURE_STORAGE_KEY
// (_validators.py:163-172).
func ResolveInputs(accountName, accountKey, connectionString string) Creds {
	if connectionString == "" && accountKey == "" {
		connectionString = os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
	}
	if connectionString != "" {
		parts := ParseConnectionString(connectionString)
		accountName = parts["AccountName"]
		accountKey = parts["AccountKey"]
	}
	if accountName == "" {
		accountName = os.Getenv("AZURE_STORAGE_ACCOUNT")
	}
	if accountKey == "" {
		accountKey = os.Getenv("AZURE_STORAGE_KEY")
	}
	return Creds{AccountName: accountName, AccountKey: accountKey}
}

// FetchAccountKey looks up a storage account's first key over ARM. The
// resource group is discovered by listing the subscription's storage accounts
// and matching on name, so the user does not have to pass -g.
func FetchAccountKey(ctx context.Context, accountName string) (string, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return "", err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return "", err
	}
	client, err := armstorage.NewAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create storage accounts client: %w", err)
	}

	resourceGroup := ""
	pager := client.NewListPager(nil)
	for pager.More() && resourceGroup == "" {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list storage accounts: %w", err)
		}
		for _, acct := range page.Value {
			if acct.Name != nil && *acct.Name == accountName && acct.ID != nil {
				resourceGroup = resourceGroupFromID(*acct.ID)
				break
			}
		}
	}
	if resourceGroup == "" {
		return "", fmt.Errorf("storage account %q was not found in the current subscription", accountName)
	}

	resp, err := client.ListKeys(ctx, resourceGroup, accountName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list keys for storage account %q: %w", accountName, err)
	}
	for _, k := range resp.Keys {
		if k.Value != nil && *k.Value != "" {
			return *k.Value, nil
		}
	}
	return "", fmt.Errorf("storage account %q returned no keys", accountName)
}

// resourceGroupFromID pulls the resource group out of an ARM resource ID.
func resourceGroupFromID(id string) string {
	parts := strings.Split(id, "/")
	for i, p := range parts {
		if strings.EqualFold(p, "resourceGroups") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// Resolve runs the full chain, falling back to an ARM key lookup when a name
// is known but no key was supplied. Unlike azure-cli, which warns and
// continues on lookup failure, this returns the missing-credentials message
// with the lookup failure appended.
func Resolve(ctx context.Context, accountName, accountKey, connectionString string) (Creds, error) {
	creds := ResolveInputs(accountName, accountKey, connectionString)
	if creds.AccountName == "" {
		return creds, fmt.Errorf("%s", MissingCredentialsError)
	}
	if creds.AccountKey != "" {
		return creds, nil
	}

	fmt.Fprintln(os.Stderr, "There are no credentials provided in your command and environment, we will query for account key for your storage account.")
	fmt.Fprintln(os.Stderr, "It is recommended to provide --connection-string or --account-key in your command as credentials.")

	key, err := FetchAccountKey(ctx, creds.AccountName)
	if err != nil {
		return creds, fmt.Errorf("%s\n\nquerying the account key failed: %v", MissingCredentialsError, err)
	}
	creds.AccountKey = key
	return creds, nil
}
