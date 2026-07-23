package keyvault

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// Update applies a partial (PATCH) update to a vault. Only flags the user set
// are sent; unset flags leave the corresponding vault property unchanged.
func Update(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, tags map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armkeyvault.NewVaultsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key vaults client: %w", err)
	}

	props := &armkeyvault.VaultPatchProperties{}
	if cmd.Flags().Changed("enable-rbac-authorization") {
		b, _ := cmd.Flags().GetBool("enable-rbac-authorization")
		props.EnableRbacAuthorization = to.Ptr(b)
	}
	if cmd.Flags().Changed("enabled-for-deployment") {
		b, _ := cmd.Flags().GetBool("enabled-for-deployment")
		props.EnabledForDeployment = to.Ptr(b)
	}
	if cmd.Flags().Changed("enabled-for-disk-encryption") {
		b, _ := cmd.Flags().GetBool("enabled-for-disk-encryption")
		props.EnabledForDiskEncryption = to.Ptr(b)
	}
	if cmd.Flags().Changed("enabled-for-template-deployment") {
		b, _ := cmd.Flags().GetBool("enabled-for-template-deployment")
		props.EnabledForTemplateDeployment = to.Ptr(b)
	}

	parameters := armkeyvault.VaultPatchParameters{Properties: props}
	if len(tags) > 0 {
		azureTags := make(map[string]*string, len(tags))
		for k, v := range tags {
			azureTags[k] = to.Ptr(v)
		}
		parameters.Tags = azureTags
	}

	resp, err := client.Update(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to update key vault: %w", err)
	}
	return output.PrintJSON(cmd, resp.Vault)
}
