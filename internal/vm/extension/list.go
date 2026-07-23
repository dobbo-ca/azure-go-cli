package extension

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup, vmName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineExtensionsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create vm extensions client: %w", err)
	}

	resp, err := client.List(ctx, resourceGroup, vmName, nil)
	if err != nil {
		return fmt.Errorf("failed to list vm extensions: %w", err)
	}
	return output.PrintJSON(cmd, resp.Value)
}
