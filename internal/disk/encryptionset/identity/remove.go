package identity

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func removeIdentity(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, system bool, userAssigned []string) error {
	if !system && len(userAssigned) == 0 {
		return fmt.Errorf("specify --system-assigned and/or --user-assigned")
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	current, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get disk encryption set: %w", err)
	}

	// Determine what remains after removal so the identity type can be recomputed.
	hasSystem := false
	remaining := map[string]struct{}{}
	if current.Identity != nil {
		if current.Identity.Type != nil {
			t := string(*current.Identity.Type)
			hasSystem = t == "SystemAssigned" || t == "SystemAssigned, UserAssigned"
		}
		for id := range current.Identity.UserAssignedIdentities {
			remaining[id] = struct{}{}
		}
	}
	if system {
		hasSystem = false
	}

	// Send each removed user-assigned identity as a nil map value (ARM removal
	// convention) and drop it from the remaining set.
	update := map[string]*armcompute.UserAssignedIdentitiesValue{}
	for _, id := range userAssigned {
		update[id] = nil
		delete(remaining, id)
	}

	identity := &armcompute.EncryptionSetIdentity{
		Type: to.Ptr(armcompute.DiskEncryptionSetIdentityType(identityType(hasSystem, len(remaining) > 0))),
	}
	if len(update) > 0 {
		identity.UserAssignedIdentities = update
	}

	fmt.Printf("Removing identity from disk encryption set '%s'...\n", name)
	poller, err := client.BeginUpdate(ctx, resourceGroup, name, armcompute.DiskEncryptionSetUpdate{Identity: identity}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update disk encryption set: %w", err)
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to remove identity: %w", err)
	}

	return output.PrintJSON(cmd, resp.Identity)
}
