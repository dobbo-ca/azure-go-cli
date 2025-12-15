package container

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, accountName, containerName, resourceGroup, publicAccess string, metadata map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armstorage.NewBlobContainersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create blob containers client: %w", err)
	}

	// Convert metadata to Azure format
	azureMetadata := make(map[string]*string)
	for k, v := range metadata {
		azureMetadata[k] = to.Ptr(v)
	}

	// Parse public access level
	var access armstorage.PublicAccess
	switch publicAccess {
	case "None", "none", "":
		access = armstorage.PublicAccessNone
	case "Container", "container":
		access = armstorage.PublicAccessContainer
	case "Blob", "blob":
		access = armstorage.PublicAccessBlob
	default:
		return fmt.Errorf("invalid public access level: %s (must be None, Container, or Blob)", publicAccess)
	}

	parameters := armstorage.BlobContainer{
		ContainerProperties: &armstorage.ContainerProperties{
			PublicAccess: to.Ptr(access),
			Metadata:     azureMetadata,
		},
	}

	fmt.Printf("Creating blob container '%s'...\n", containerName)
	result, err := client.Create(ctx, resourceGroup, accountName, containerName, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to create blob container: %w", err)
	}

	return output.PrintJSON(cmd, result.BlobContainer)
}
