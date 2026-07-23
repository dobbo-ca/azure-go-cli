package nodepool

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

func DeleteMachines(ctx context.Context, cmd *cobra.Command, clusterName, nodepoolName, resourceGroup string, machineNames []string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewAgentPoolsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create agent pools client: %w", err)
	}

	names := make([]*string, 0, len(machineNames))
	for _, n := range machineNames {
		names = append(names, to.Ptr(n))
	}
	param := armcontainerservice.AgentPoolDeleteMachinesParameter{
		MachineNames: names,
	}

	fmt.Printf("Deleting machines from node pool '%s'...\n", nodepoolName)

	poller, err := client.BeginDeleteMachines(ctx, resourceGroup, clusterName, nodepoolName, param, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete machines: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "delete machines started"})
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete machines failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("deleted machines from node pool '%s'.", nodepoolName)})
}
