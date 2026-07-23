package vmss

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func SetOrchestrationServiceState(ctx context.Context, cmd *cobra.Command, resourceGroup, name, serviceName, action string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineScaleSetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VMSS client: %w", err)
	}

	fmt.Printf("Setting orchestration service state on scale set '%s'...\n", name)
	poller, err := client.BeginSetOrchestrationServiceState(ctx, resourceGroup, name, armcompute.OrchestrationServiceStateInput{
		ServiceName: to.Ptr(armcompute.OrchestrationServiceNames(serviceName)),
		Action:      to.Ptr(armcompute.OrchestrationServiceStateAction(action)),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin set orchestration service state: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "set orchestration service state started"})
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("set orchestration service state failed: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("orchestration service state set on '%s'.", name)})
}
