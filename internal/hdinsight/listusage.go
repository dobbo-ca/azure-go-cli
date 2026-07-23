package hdinsight

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func ListUsage(ctx context.Context, cmd *cobra.Command, location string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armhdinsight.NewLocationsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create locations client: %w", err)
	}

	resp, err := client.ListUsages(ctx, location, nil)
	if err != nil {
		return fmt.Errorf("failed to list usages: %w", err)
	}

	return output.PrintJSON(cmd, resp.UsagesListResult)
}
