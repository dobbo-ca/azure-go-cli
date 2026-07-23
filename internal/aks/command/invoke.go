package command

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Invoke(ctx context.Context, cmd *cobra.Command, name, resourceGroup, command string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create managed clusters client: %w", err)
	}

	req := armcontainerservice.RunCommandRequest{
		Command: to.Ptr(command),
	}

	fmt.Printf("Running command against cluster '%s'...\n", name)
	poller, err := client.BeginRunCommand(ctx, resourceGroup, name, req, nil)
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
