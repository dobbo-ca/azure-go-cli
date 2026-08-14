package blob

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, accountName, containerName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	// Build blob service URL
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)

	client, err := azblob.NewClient(serviceURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create blob client: %w", err)
	}

	var blobs []*container.BlobItem
	pager := client.NewListBlobsFlatPager(containerName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list blobs: %w", err)
		}

		if page.Segment == nil || page.Segment.BlobItems == nil {
			continue
		}
		blobs = append(blobs, page.Segment.BlobItems...)
	}

	// blob list historically defaulted to table output, unlike the global
	// default of json, so only honor an explicitly-passed format.
	format, _ := cmd.Flags().GetString("output")
	if !cmd.Flags().Changed("output") {
		format = "table"
	}
	if format != "table" {
		return output.PrintJSON(cmd, blobs)
	}

	fmt.Printf("%-60s %-20s %-15s %-20s\n", "NAME", "BLOB TYPE", "SIZE (bytes)", "LAST MODIFIED")
	fmt.Println("-------------------------------------------------------------------------------------------------------------------------------")
	for _, blobItem := range blobs {
		printBlob(blobItem)
	}

	return nil
}

func printBlob(blob *container.BlobItem) {
	name := ""
	if blob.Name != nil {
		name = *blob.Name
	}

	blobType := ""
	if blob.Properties != nil && blob.Properties.BlobType != nil {
		blobType = string(*blob.Properties.BlobType)
	}

	size := ""
	if blob.Properties != nil && blob.Properties.ContentLength != nil {
		size = fmt.Sprintf("%d", *blob.Properties.ContentLength)
	}

	lastModified := ""
	if blob.Properties != nil && blob.Properties.LastModified != nil {
		lastModified = blob.Properties.LastModified.Format("2006-01-02 15:04:05")
	}

	fmt.Printf("%-60s %-20s %-15s %-20s\n", name, blobType, size, lastModified)
}
