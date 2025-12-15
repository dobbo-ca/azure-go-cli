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

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location string, tags map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armkeyvault.NewVaultsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key vaults client: %w", err)
	}

	// Convert tags to Azure format
	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	// Get tenant ID from subscription
	tenantID, err := config.GetTenantID(subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to get tenant ID: %w", err)
	}

	parameters := armkeyvault.VaultCreateOrUpdateParameters{
		Location: to.Ptr(location),
		Tags:     azureTags,
		Properties: &armkeyvault.VaultProperties{
			TenantID: to.Ptr(tenantID),
			SKU: &armkeyvault.SKU{
				Family: to.Ptr(armkeyvault.SKUFamilyA),
				Name:   to.Ptr(armkeyvault.SKUNameStandard),
			},
			AccessPolicies:               []*armkeyvault.AccessPolicyEntry{},
			EnabledForDeployment:         to.Ptr(false),
			EnabledForDiskEncryption:     to.Ptr(false),
			EnabledForTemplateDeployment: to.Ptr(false),
			EnableSoftDelete:             to.Ptr(true),
			SoftDeleteRetentionInDays:    to.Ptr[int32](90),
			EnableRbacAuthorization:      to.Ptr(true),
		},
	}

	fmt.Printf("Creating key vault '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create key vault: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create key vault: %w", err)
	}

	return output.PrintJSON(cmd, result.Vault)
}
