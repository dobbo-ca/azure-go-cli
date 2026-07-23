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

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a disk encryption set",
		Long:  "Update the active key, key rotation, encryption type, or tags of a disk encryption set. Only the fields you pass are changed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return updateDiskEncryptionSet(context.Background(), cmd, name, resourceGroup, noWait)
		},
	}

	cmd.Flags().StringP("name", "n", "", "Disk encryption set name")
	cmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	cmd.Flags().String("key-url", "", "New fully versioned Key Vault key URL")
	cmd.Flags().String("source-vault", "", "Resource ID of the Key Vault containing the key")
	cmd.Flags().String("encryption-type", "", "Encryption type: EncryptionAtRestWithCustomerKey, EncryptionAtRestWithPlatformAndCustomerKeys, or ConfidentialVmEncryptedWithCustomerKey")
	cmd.Flags().Bool("enable-auto-key-rotation", false, "Enable auto-rotation to the latest key version")
	cmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	cmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("resource-group")

	return cmd
}

func updateDiskEncryptionSet(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, noWait bool) error {
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

	props := &armcompute.DiskEncryptionSetUpdateProperties{}
	flags := cmd.Flags()

	if flags.Changed("key-url") {
		keyURL, _ := flags.GetString("key-url")
		activeKey := &armcompute.KeyForDiskEncryptionSet{KeyURL: to.Ptr(keyURL)}
		if flags.Changed("source-vault") {
			sourceVault, _ := flags.GetString("source-vault")
			activeKey.SourceVault = &armcompute.SourceVault{ID: to.Ptr(sourceVault)}
		}
		props.ActiveKey = activeKey
	}
	if flags.Changed("encryption-type") {
		encryptionType, _ := flags.GetString("encryption-type")
		props.EncryptionType = to.Ptr(armcompute.DiskEncryptionSetType(encryptionType))
	}
	if flags.Changed("enable-auto-key-rotation") {
		rotate, _ := flags.GetBool("enable-auto-key-rotation")
		props.RotationToLatestKeyVersionEnabled = to.Ptr(rotate)
	}

	update := armcompute.DiskEncryptionSetUpdate{Properties: props}

	if flags.Changed("tags") {
		tags, _ := flags.GetStringToString("tags")
		azureTags := make(map[string]*string)
		for k, v := range tags {
			azureTags[k] = to.Ptr(v)
		}
		update.Tags = azureTags
	}

	fmt.Printf("Updating disk encryption set '%s'...\n", name)
	poller, err := client.BeginUpdate(ctx, resourceGroup, name, update, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update disk encryption set: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Update of disk encryption set '%s' started.", name)})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update disk encryption set: %w", err)
	}

	return output.PrintJSON(cmd, resp.DiskEncryptionSet)
}
