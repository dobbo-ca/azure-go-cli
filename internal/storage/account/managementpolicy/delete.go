package managementpolicy

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Delete(ctx context.Context, cmd *cobra.Command, account, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armstorage.NewManagementPoliciesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create management policies client: %w", err)
	}

	if _, err := client.Delete(ctx, resourceGroup, account, armstorage.ManagementPolicyNameDefault, nil); err != nil {
		return fmt.Errorf("failed to delete management policy: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": "management policy deleted."})
}
