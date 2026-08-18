package identity

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func assignIdentity(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, system bool, userAssigned []string) error {
	if !system && len(userAssigned) == 0 {
		return fmt.Errorf("specify --system-assigned and/or --user-assigned")
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	identity := &armcompute.EncryptionSetIdentity{
		Type: to.Ptr(armcompute.DiskEncryptionSetIdentityType(identityType(system, len(userAssigned) > 0))),
	}
	if len(userAssigned) > 0 {
		ids := make(map[string]*armcompute.UserAssignedIdentitiesValue, len(userAssigned))
		for _, id := range userAssigned {
			ids[id] = &armcompute.UserAssignedIdentitiesValue{}
		}
		identity.UserAssignedIdentities = ids
	}

	fmt.Printf("Assigning identity to disk encryption set '%s'...\n", name)
	poller, err := client.BeginUpdate(ctx, resourceGroup, name, armcompute.DiskEncryptionSetUpdate{Identity: identity}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update disk encryption set: %w", err)
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to assign identity: %w", err)
	}

	return output.PrintJSON(cmd, resp.Identity)
}
