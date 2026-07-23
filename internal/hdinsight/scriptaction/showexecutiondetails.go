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

func ShowExecutionDetails(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, executionID string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	actions, err := armhdinsight.NewScriptActionsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create script actions client: %w", err)
	}

	resp, err := actions.GetExecutionDetail(ctx, resourceGroup, clusterName, executionID, nil)
	if err != nil {
		return fmt.Errorf("failed to get script execution detail: %w", err)
	}

	return output.PrintJSON(cmd, resp.RuntimeScriptActionDetail)
}
