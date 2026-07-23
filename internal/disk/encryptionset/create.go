package encryptionset

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a disk encryption set",
		Long:  "Create a disk encryption set that uses a Key Vault key to encrypt managed disks. A system-assigned managed identity is created by default and must be granted access to the key vault.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			keyURL, _ := cmd.Flags().GetString("key-url")
			sourceVault, _ := cmd.Flags().GetString("source-vault")
			encryptionType, _ := cmd.Flags().GetString("encryption-type")
			tags, _ := cmd.Flags().GetStringToString("tags")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return createDiskEncryptionSet(context.Background(), cmd, name, resourceGroup, location, keyURL, sourceVault, encryptionType, tags, noWait)
		},
	}

	cmd.Flags().StringP("name", "n", "", "Disk encryption set name")
	cmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	cmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	cmd.Flags().String("key-url", "", "Fully versioned Key Vault key URL used for encryption")
	cmd.Flags().String("source-vault", "", "Resource ID of the Key Vault containing the key (optional)")
	cmd.Flags().String("encryption-type", "EncryptionAtRestWithCustomerKey", "Encryption type: EncryptionAtRestWithCustomerKey, EncryptionAtRestWithPlatformAndCustomerKeys, or ConfidentialVmEncryptedWithCustomerKey")
	cmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	cmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("resource-group")
	cmd.MarkFlagRequired("location")
	cmd.MarkFlagRequired("key-url")

	return cmd
}

func createDiskEncryptionSet(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, keyURL, sourceVault, encryptionType string, tags map[string]string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armcompute.NewDiskEncryptionSetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create disk encryption sets client: %w", err)
	}

	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	activeKey := &armcompute.KeyForDiskEncryptionSet{
		KeyURL: to.Ptr(keyURL),
	}
	if sourceVault != "" {
		activeKey.SourceVault = &armcompute.SourceVault{ID: to.Ptr(sourceVault)}
	}

	params := armcompute.DiskEncryptionSet{
		Location: to.Ptr(location),
		Tags:     azureTags,
		Identity: &armcompute.EncryptionSetIdentity{
			Type: to.Ptr(armcompute.DiskEncryptionSetIdentityTypeSystemAssigned),
		},
		Properties: &armcompute.EncryptionSetProperties{
			ActiveKey:      activeKey,
			EncryptionType: to.Ptr(armcompute.DiskEncryptionSetType(encryptionType)),
		},
	}

	fmt.Printf("Creating disk encryption set '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, params, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create disk encryption set: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Create of disk encryption set '%s' started.", name)})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create disk encryption set: %w", err)
	}

	return output.PrintJSON(cmd, resp.DiskEncryptionSet)
}
