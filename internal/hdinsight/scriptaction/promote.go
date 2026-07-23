package scriptaction

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Promote(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, executionID string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	history, err := armhdinsight.NewScriptExecutionHistoryClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create script execution history client: %w", err)
	}

	_, err = history.Promote(ctx, resourceGroup, clusterName, executionID, nil)
	if err != nil {
		return fmt.Errorf("failed to promote script execution: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("execution '%s' promoted.", executionID)})
}
