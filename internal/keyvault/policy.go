package keyvault

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// PolicyOptions carries the flags of az keyvault set-policy and delete-policy
// (custom.py:790 and custom.py:1055).
type PolicyOptions struct {
	VaultName              string
	ResourceGroup          string
	ObjectID               string
	ApplicationID          string
	SPN                    string
	UPN                    string
	KeyPermissions         []string
	SecretPermissions      []string
	CertificatePermissions []string
	StoragePermissions     []string
}

func vaultsClient() (*armkeyvault.VaultsClient, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return nil, err
	}
	client, err := armkeyvault.NewVaultsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create key vaults client: %w", err)
	}
	return client, nil
}

func permissionList[T ~string](values []string) []*T {
	if values == nil {
		return nil
	}
	out := make([]*T, 0, len(values))
	for _, v := range distinct(values) {
		out = append(out, to.Ptr(T(v)))
	}
	return out
}

// SetPolicy adds or updates one access policy entry on a vault.
func SetPolicy(ctx context.Context, cmd *cobra.Command, opts PolicyOptions) error {
	objectID, err := resolveObjectID(ctx, opts.ObjectID, opts.SPN, opts.UPN)
	if err != nil {
		return err
	}
	client, err := vaultsClient()
	if err != nil {
		return err
	}
	vault, err := client.Get(ctx, opts.ResourceGroup, opts.VaultName, nil)
	if err != nil {
		return fmt.Errorf("failed to get key vault: %w", err)
	}
	props := vault.Properties
	if props.EnableRbacAuthorization != nil && *props.EnableRbacAuthorization {
		return fmt.Errorf("cannot set policies to a vault with '--enable-rbac-authorization' specified")
	}

	existing := findPolicy(props, objectID, opts.ApplicationID)
	if existing == nil {
		entry := &armkeyvault.AccessPolicyEntry{
			TenantID: props.TenantID,
			ObjectID: to.Ptr(objectID),
			Permissions: &armkeyvault.Permissions{
				Keys:         permissionList[armkeyvault.KeyPermissions](opts.KeyPermissions),
				Secrets:      permissionList[armkeyvault.SecretPermissions](opts.SecretPermissions),
				Certificates: permissionList[armkeyvault.CertificatePermissions](opts.CertificatePermissions),
				Storage:      permissionList[armkeyvault.StoragePermissions](opts.StoragePermissions),
			},
		}
		if opts.ApplicationID != "" {
			entry.ApplicationID = to.Ptr(opts.ApplicationID)
		}
		props.AccessPolicies = append(props.AccessPolicies, entry)
	} else {
		// A permission list the caller left out keeps its previous value
		// (custom.py:834).
		if opts.KeyPermissions != nil {
			existing.Permissions.Keys = permissionList[armkeyvault.KeyPermissions](opts.KeyPermissions)
		}
		if opts.SecretPermissions != nil {
			existing.Permissions.Secrets = permissionList[armkeyvault.SecretPermissions](opts.SecretPermissions)
		}
		if opts.CertificatePermissions != nil {
			existing.Permissions.Certificates = permissionList[armkeyvault.CertificatePermissions](opts.CertificatePermissions)
		}
		if opts.StoragePermissions != nil {
			existing.Permissions.Storage = permissionList[armkeyvault.StoragePermissions](opts.StoragePermissions)
		}
	}

	return updateVault(ctx, cmd, client, opts.ResourceGroup, opts.VaultName, vault.Vault)
}

// DeletePolicy removes one access policy entry from a vault.
func DeletePolicy(ctx context.Context, cmd *cobra.Command, opts PolicyOptions) error {
	objectID, err := resolveObjectID(ctx, opts.ObjectID, opts.SPN, opts.UPN)
	if err != nil {
		return err
	}
	client, err := vaultsClient()
	if err != nil {
		return err
	}
	vault, err := client.Get(ctx, opts.ResourceGroup, opts.VaultName, nil)
	if err != nil {
		return fmt.Errorf("failed to get key vault: %w", err)
	}
	props := vault.Properties
	if props.EnableRbacAuthorization != nil && *props.EnableRbacAuthorization {
		return fmt.Errorf("cannot delete policies to a vault with '--enable-rbac-authorization' specified")
	}

	kept := make([]*armkeyvault.AccessPolicyEntry, 0, len(props.AccessPolicies))
	for _, policy := range props.AccessPolicies {
		if !policyMatches(props, policy, objectID, opts.ApplicationID) {
			kept = append(kept, policy)
		}
	}
	if len(kept) == len(props.AccessPolicies) {
		return fmt.Errorf("no matching policies found")
	}
	props.AccessPolicies = kept

	return updateVault(ctx, cmd, client, opts.ResourceGroup, opts.VaultName, vault.Vault)
}

// policyMatches compares a policy entry the way set_policy and delete_policy
// do: object id, application id and tenant id, all case-insensitively
// (custom.py:817 and custom.py:1073).
func policyMatches(props *armkeyvault.VaultProperties, policy *armkeyvault.AccessPolicyEntry, objectID, applicationID string) bool {
	if policy == nil || policy.ObjectID == nil {
		return false
	}
	if !strings.EqualFold(*policy.ObjectID, objectID) {
		return false
	}
	policyApp := ""
	if policy.ApplicationID != nil {
		policyApp = *policy.ApplicationID
	}
	if !strings.EqualFold(policyApp, applicationID) {
		return false
	}
	if props.TenantID == nil || policy.TenantID == nil {
		return false
	}
	return strings.EqualFold(*props.TenantID, *policy.TenantID)
}

func findPolicy(props *armkeyvault.VaultProperties, objectID, applicationID string) *armkeyvault.AccessPolicyEntry {
	for _, policy := range props.AccessPolicies {
		if policyMatches(props, policy, objectID, applicationID) {
			if policy.Permissions == nil {
				policy.Permissions = &armkeyvault.Permissions{}
			}
			return policy
		}
	}
	return nil
}

// updateVault writes the vault back, as every one of these handlers does.
func updateVault(ctx context.Context, cmd *cobra.Command, client *armkeyvault.VaultsClient, resourceGroup, vaultName string, vault armkeyvault.Vault) error {
	params := armkeyvault.VaultCreateOrUpdateParameters{
		Location:   vault.Location,
		Tags:       vault.Tags,
		Properties: vault.Properties,
	}
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, vaultName, params, nil)
	if err != nil {
		return fmt.Errorf("failed to update key vault: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update key vault: %w", err)
	}
	return output.PrintJSON(cmd, resp.Vault)
}
