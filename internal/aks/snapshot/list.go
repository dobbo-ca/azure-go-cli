package snapshot

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup string) error {
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

	var snapshots []map[string]interface{}

	if resourceGroup != "" {
		// List snapshots in specific resource group
		pager := client.NewListByResourceGroupPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list snapshots: %w", err)
			}

			for _, snapshot := range page.Value {
				snapshots = append(snapshots, formatSnapshot(snapshot))
			}
		}
	} else {
		// List all snapshots in subscription
		pager := client.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list snapshots: %w", err)
			}

			for _, snapshot := range page.Value {
				snapshots = append(snapshots, formatSnapshot(snapshot))
			}
		}
	}

	return output.PrintJSON(cmd, snapshots)
}

func formatSnapshot(snapshot *armcontainerservice.Snapshot) map[string]interface{} {
	result := map[string]interface{}{
		"name":     azure.GetStringValue(snapshot.Name),
		"location": azure.GetStringValue(snapshot.Location),
	}

	if snapshot.Properties != nil {
		if snapshot.Properties.CreationData != nil && snapshot.Properties.CreationData.SourceResourceID != nil {
			result["sourceResourceId"] = *snapshot.Properties.CreationData.SourceResourceID
		}
		if snapshot.Properties.KubernetesVersion != nil {
			result["kubernetesVersion"] = *snapshot.Properties.KubernetesVersion
		}
		if snapshot.Properties.NodeImageVersion != nil {
			result["nodeImageVersion"] = *snapshot.Properties.NodeImageVersion
		}
		if snapshot.Properties.OSType != nil {
			result["osType"] = string(*snapshot.Properties.OSType)
		}
	}

	return result
}
