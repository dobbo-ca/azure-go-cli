package vmss

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func UpdateDomainWalk(ctx context.Context, cmd *cobra.Command, resourceGroup, name string, platformUpdateDomain int32) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineScaleSetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VMSS client: %w", err)
	}

	resp, err := client.ForceRecoveryServiceFabricPlatformUpdateDomainWalk(ctx, resourceGroup, name, platformUpdateDomain, nil)
	if err != nil {
		return fmt.Errorf("failed to force recovery update domain walk: %w", err)
	}
	return output.PrintJSON(cmd, resp.RecoveryWalkResponse)
}
