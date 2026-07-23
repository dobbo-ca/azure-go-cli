package runcommand

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

func Invoke(ctx context.Context, cmd *cobra.Command, resourceGroup, vmName, commandID, script string, noWait bool) error {
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
		return fmt.Errorf("failed to create virtual machines client: %w", err)
	}

	input := armcompute.RunCommandInput{
		CommandID: to.Ptr(commandID),
	}
	if script != "" {
		input.Script = []*string{to.Ptr(script)}
	}

	fmt.Printf("Invoking run command '%s' on VM '%s'...\n", commandID, vmName)
	poller, err := client.BeginRunCommand(ctx, resourceGroup, vmName, input, nil)
	if err != nil {
		return fmt.Errorf("failed to begin run command: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "run command started"})
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("run command failed: %w", err)
	}
	return output.PrintJSON(cmd, resp.RunCommandResult)
}
