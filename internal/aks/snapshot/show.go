package snapshot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, snapshotName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewSnapshotsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create snapshots client: %w", err)
	}

	snapshot, err := client.Get(ctx, resourceGroup, snapshotName, nil)
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format snapshot: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
