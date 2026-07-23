package lock

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// Update changes the level and/or notes of an existing subscription-level lock.
// Only flags that were explicitly provided are applied; other fields are
// preserved from the current lock.
func Update(ctx context.Context, cmd *cobra.Command, name, lockType, notes string) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	current, err := client.GetAtSubscriptionLevel(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get lock %q: %w", name, err)
	}

	props := current.Properties
	if props == nil {
		props = &armlocks.ManagementLockProperties{}
	}

	if lockType != "" {
		level, err := parseLockLevel(lockType)
		if err != nil {
			return err
		}
		props.Level = &level
	}
	if cmd.Flags().Changed("notes") {
		props.Notes = to.Ptr(notes)
	}

	resp, err := client.CreateOrUpdateAtSubscriptionLevel(ctx, name, armlocks.ManagementLockObject{
		Properties: props,
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to update lock %q: %w", name, err)
	}

	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, lockToInfo(&resp.ManagementLockObject), format)
}
