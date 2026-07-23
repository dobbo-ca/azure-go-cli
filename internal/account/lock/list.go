package lock

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// List returns all subscription-level management locks.
func List(ctx context.Context, cmd *cobra.Command) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	locks := []LockInfo{}
	pager := client.NewListAtSubscriptionLevelPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list locks: %w", err)
		}
		for _, l := range page.Value {
			locks = append(locks, lockToInfo(l))
		}
	}

	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, locks, format)
}
