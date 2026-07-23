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

func Remove(ctx context.Context, cmd *cobra.Command, resourceGroup, name string, identityIDs []string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual machines client: %w", err)
	}

	userAssigned := make(map[string]*armcompute.UserAssignedIdentitiesValue, len(identityIDs))
	for _, id := range identityIDs {
		userAssigned[id] = nil
	}
	identity := &armcompute.VirtualMachineIdentity{
		Type:                   to.Ptr(armcompute.ResourceIdentityTypeUserAssigned),
		UserAssignedIdentities: userAssigned,
	}

	fmt.Printf("Removing identities from '%s'...\n", name)
	poller, err := client.BeginUpdate(ctx, resourceGroup, name, armcompute.VirtualMachineUpdate{
		Identity: identity,
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "identity removal started"})
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("identity removal failed: %w", err)
	}
	return output.PrintJSON(cmd, resp.VirtualMachine.Identity)
}
