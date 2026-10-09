package image

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func ListSKUs(ctx context.Context, cmd *cobra.Command, location, publisher, offer string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subID, err := subscriptionID(cmd)
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineImagesClient(subID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create image client: %w", err)
	}
	res, err := client.ListSKUs(ctx, location, publisher, offer, nil)
	if err != nil {
		return fmt.Errorf("failed to list image SKUs: %w", err)
	}
	return output.PrintJSON(cmd, res.VirtualMachineImageResourceArray)
}
