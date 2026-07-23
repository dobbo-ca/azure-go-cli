package managementgroup

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/managementgroups/armmanagementgroups"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

func newSubscriptionsClient() (*armmanagementgroups.ManagementGroupSubscriptionsClient, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}
	return armmanagementgroups.NewManagementGroupSubscriptionsClient(cred, nil)
}

// AddSubscription associates a subscription with a management group.
func AddSubscription(ctx context.Context, name, subscription string) error {
	client, err := newSubscriptionsClient()
	if err != nil {
		return err
	}
	if _, err := client.Create(ctx, name, subscription, nil); err != nil {
		return fmt.Errorf("failed to add subscription %q to management group %q: %w", subscription, name, err)
	}
	fmt.Printf("Added subscription %s to management group '%s'\n", subscription, name)
	return nil
}

// RemoveSubscription dissociates a subscription from a management group.
func RemoveSubscription(ctx context.Context, name, subscription string) error {
	client, err := newSubscriptionsClient()
	if err != nil {
		return err
	}
	if _, err := client.Delete(ctx, name, subscription, nil); err != nil {
		return fmt.Errorf("failed to remove subscription %q from management group %q: %w", subscription, name, err)
	}
	fmt.Printf("Removed subscription %s from management group '%s'\n", subscription, name)
	return nil
}
