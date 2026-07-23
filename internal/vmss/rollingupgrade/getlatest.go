package rollingupgrade

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func GetLatest(ctx context.Context, cmd *cobra.Command, resourceGroup, vmssName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineScaleSetRollingUpgradesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create rolling upgrades client: %w", err)
	}

	resp, err := client.GetLatest(ctx, resourceGroup, vmssName, nil)
	if err != nil {
		return fmt.Errorf("failed to get latest rolling upgrade: %w", err)
	}
	return output.PrintJSON(cmd, resp.RollingUpgradeStatusInfo)
}
