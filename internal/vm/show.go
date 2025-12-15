package vm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, name, resourceGroup string) error {
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

	result, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get VM: %w", err)
	}

	jsonData, err := json.MarshalIndent(result.VirtualMachine, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal VM data: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}
