package host

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Restart(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName string, hostNames []string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armhdinsight.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual machines client: %w", err)
	}

	hosts := make([]*string, 0, len(hostNames))
	for _, h := range hostNames {
		hosts = append(hosts, to.Ptr(h))
	}

	fmt.Printf("Restarting %d host(s) on cluster '%s'...\n", len(hostNames), clusterName)
	poller, err := client.BeginRestartHosts(ctx, resourceGroup, clusterName, hosts, nil)
	if err != nil {
		return fmt.Errorf("failed to begin restart hosts: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "restart started"})
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("restart hosts operation failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("restarted %d host(s).", len(hostNames))})
}
