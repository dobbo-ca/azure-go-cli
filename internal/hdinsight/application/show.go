package application

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, applicationName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armhdinsight.NewApplicationsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create applications client: %w", err)
	}

	resp, err := client.Get(ctx, resourceGroup, clusterName, applicationName, nil)
	if err != nil {
		return fmt.Errorf("failed to get application: %w", err)
	}

	return output.PrintJSON(cmd, resp.Application)
}
