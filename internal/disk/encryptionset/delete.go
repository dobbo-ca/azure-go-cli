package encryptionset

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a disk encryption set",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return deleteDiskEncryptionSet(context.Background(), cmd, name, resourceGroup, noWait)
		},
	}

	cmd.Flags().StringP("name", "n", "", "Disk encryption set name")
	cmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	cmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("resource-group")

	return cmd
}

func deleteDiskEncryptionSet(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, noWait bool) error {
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

	poller, err := client.BeginDelete(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete disk encryption set: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Delete of disk encryption set '%s' started.", name)})
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to delete disk encryption set: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Disk encryption set '%s' deleted.", name)})
}
