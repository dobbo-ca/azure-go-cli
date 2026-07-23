package oidcissuer

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func RotateSigningKeys(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create managed clusters client: %w", err)
	}

	fmt.Printf("Rotating service account signing keys for '%s'...\n", name)
	poller, err := client.BeginRotateServiceAccountSigningKeys(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin rotate service account signing keys: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "rotate service account signing keys started"})
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("rotate service account signing keys operation failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("service account signing keys rotated for '%s'.", name)})
}
