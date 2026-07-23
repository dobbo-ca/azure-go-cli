package blobservice

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, account, resourceGroup string, enableDeleteRetention *bool, deleteRetentionDays *int32, enableVersioning *bool, enableChangeFeed *bool, enableContainerDeleteRetention *bool, containerDeleteRetentionDays *int32) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armstorage.NewBlobServicesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create blob services client: %w", err)
	}

	cur, err := client.GetServiceProperties(ctx, resourceGroup, account, nil)
	if err != nil {
		return fmt.Errorf("failed to get blob service properties: %w", err)
	}
	if cur.BlobServiceProperties.BlobServiceProperties == nil {
		cur.BlobServiceProperties.BlobServiceProperties = &armstorage.BlobServicePropertiesProperties{}
	}
	props := cur.BlobServiceProperties.BlobServiceProperties

	if enableDeleteRetention != nil || deleteRetentionDays != nil {
		if props.DeleteRetentionPolicy == nil {
			props.DeleteRetentionPolicy = &armstorage.DeleteRetentionPolicy{}
		}
		if enableDeleteRetention != nil {
			props.DeleteRetentionPolicy.Enabled = enableDeleteRetention
		}
		if deleteRetentionDays != nil {
			props.DeleteRetentionPolicy.Days = deleteRetentionDays
		}
	}

	if enableContainerDeleteRetention != nil || containerDeleteRetentionDays != nil {
		if props.ContainerDeleteRetentionPolicy == nil {
			props.ContainerDeleteRetentionPolicy = &armstorage.DeleteRetentionPolicy{}
		}
		if enableContainerDeleteRetention != nil {
			props.ContainerDeleteRetentionPolicy.Enabled = enableContainerDeleteRetention
		}
		if containerDeleteRetentionDays != nil {
			props.ContainerDeleteRetentionPolicy.Days = containerDeleteRetentionDays
		}
	}

	if enableVersioning != nil {
		props.IsVersioningEnabled = enableVersioning
	}

	if enableChangeFeed != nil {
		props.ChangeFeed = &armstorage.ChangeFeed{Enabled: enableChangeFeed}
	}

	resp, err := client.SetServiceProperties(ctx, resourceGroup, account, cur.BlobServiceProperties, nil)
	if err != nil {
		return fmt.Errorf("failed to set blob service properties: %w", err)
	}
	return output.PrintJSON(cmd, resp.BlobServiceProperties)
}
