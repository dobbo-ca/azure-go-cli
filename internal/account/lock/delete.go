package lock

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

// Delete removes a subscription-level lock by name.
func Delete(ctx context.Context, name string) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	if _, err := client.DeleteAtSubscriptionLevel(ctx, name, nil); err != nil {
		return fmt.Errorf("failed to delete lock %q: %w", name, err)
	}

	subID, _ := config.GetDefaultSubscription()
	fmt.Printf("Deleted lock '%s' on subscription %s\n", name, subID)
	return nil
}
