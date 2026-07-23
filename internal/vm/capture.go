package vm

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

func Capture(ctx context.Context, cmd *cobra.Command, resourceGroup, name, vhdPrefix, containerName string, overwrite, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VM client: %w", err)
	}

	params := armcompute.VirtualMachineCaptureParameters{
		VhdPrefix:                to.Ptr(vhdPrefix),
		DestinationContainerName: to.Ptr(containerName),
		OverwriteVhds:            to.Ptr(overwrite),
	}

	fmt.Printf("Capturing VM '%s'...\n", name)
	poller, err := client.BeginCapture(ctx, resourceGroup, name, params, nil)
	if err != nil {
		return fmt.Errorf("failed to begin capture: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "capture started"})
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("capture failed: %w", err)
	}
	return output.PrintJSON(cmd, resp.VirtualMachineCaptureResult)
}
