package identity

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Assign(ctx context.Context, cmd *cobra.Command, resourceGroup, name string, identityIDs []string, noWait bool) error {
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
		return fmt.Errorf("failed to create virtual machine scale sets client: %w", err)
	}

	userAssigned := make(map[string]*armcompute.UserAssignedIdentitiesValue, len(identityIDs))
	for _, id := range identityIDs {
		userAssigned[id] = &armcompute.UserAssignedIdentitiesValue{}
	}

	fmt.Printf("Assigning identities to '%s'...\n", name)
	poller, err := client.BeginUpdate(ctx, resourceGroup, name, armcompute.VirtualMachineScaleSetUpdate{
		Identity: &armcompute.VirtualMachineScaleSetIdentity{
			Type:                   to.Ptr(armcompute.ResourceIdentityTypeUserAssigned),
			UserAssignedIdentities: userAssigned,
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "identity assignment started"})
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("identity assignment failed: %w", err)
	}
	return output.PrintJSON(cmd, resp.VirtualMachineScaleSet.Identity)
}
