package lock

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// Create creates (or updates) a subscription-level lock.
func Create(ctx context.Context, cmd *cobra.Command, name, lockType, notes string) error {
	level, err := parseLockLevel(lockType)
	if err != nil {
		return err
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	props := &armlocks.ManagementLockProperties{Level: &level}
	if notes != "" {
		props.Notes = to.Ptr(notes)
	}

	resp, err := client.CreateOrUpdateAtSubscriptionLevel(ctx, name, armlocks.ManagementLockObject{
		Properties: props,
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create lock %q: %w", name, err)
	}

	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, lockToInfo(&resp.ManagementLockObject), format)
}
