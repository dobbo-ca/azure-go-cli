package lock

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// Show returns a single subscription-level lock by name.
func Show(ctx context.Context, cmd *cobra.Command, name string) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	resp, err := client.GetAtSubscriptionLevel(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get lock %q: %w", name, err)
	}

	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, lockToInfo(&resp.ManagementLockObject), format)
}
