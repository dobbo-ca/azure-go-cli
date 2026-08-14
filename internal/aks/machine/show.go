package machine

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, clusterName, nodepoolName, machineName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create machines client: %w", err)
	}

	machine, err := client.Get(ctx, resourceGroup, clusterName, nodepoolName, machineName, nil)
	if err != nil {
		return fmt.Errorf("failed to get machine: %w", err)
	}

	return output.PrintJSON(cmd, machine)
}
