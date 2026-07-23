package containerrm

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// buildBlobContainer assembles a BlobContainer from the provided public-access
// and metadata values, setting only fields that were supplied.
func buildBlobContainer(publicAccess string, metadata map[string]string) armstorage.BlobContainer {
	bc := armstorage.BlobContainer{ContainerProperties: &armstorage.ContainerProperties{}}
	if publicAccess != "" && !strings.EqualFold(publicAccess, "off") {
		switch strings.ToLower(publicAccess) {
		case "blob":
			bc.ContainerProperties.PublicAccess = to.Ptr(armstorage.PublicAccessBlob)
		case "container":
			bc.ContainerProperties.PublicAccess = to.Ptr(armstorage.PublicAccessContainer)
		}
	}
	if len(metadata) > 0 {
		m := make(map[string]*string, len(metadata))
		for k, v := range metadata {
			m[k] = to.Ptr(v)
		}
		bc.ContainerProperties.Metadata = m
	}
	return bc
}

func Create(ctx context.Context, cmd *cobra.Command, resourceGroup, account, name, publicAccess string, metadata map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armstorage.NewBlobContainersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create blob containers client: %w", err)
	}

	bc := buildBlobContainer(publicAccess, metadata)
	resp, err := client.Create(ctx, resourceGroup, account, name, bc, nil)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	return output.PrintJSON(cmd, resp.BlobContainer)
}
