package nodepool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, clusterName, nodepoolName, resourceGroup string) error {
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

	pool, err := client.Get(ctx, resourceGroup, clusterName, nodepoolName, nil)
	if err != nil {
		return fmt.Errorf("failed to get node pool: %w", err)
	}

	data, err := json.MarshalIndent(pool, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format node pool: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
