package vm

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Start(ctx context.Context, name, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VM client: %w", err)
	}

	fmt.Printf("Starting virtual machine '%s'...\n", name)
	poller, err := client.BeginStart(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin start VM: %w", err)
	}

	if noWait {
		fmt.Printf("Started operation to start virtual machine '%s'\n", name)
		return nil
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	fmt.Printf("Started virtual machine '%s'\n", name)
	return nil
}
