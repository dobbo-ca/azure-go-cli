package machine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, clusterName, nodepoolName, machineName, resourceGroup string) error {
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

	data, err := json.MarshalIndent(machine, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format machine: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
