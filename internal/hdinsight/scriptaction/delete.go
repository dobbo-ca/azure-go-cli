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

func Delete(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, scriptName string) error {
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

	_, err = actions.Delete(ctx, resourceGroup, clusterName, scriptName, nil)
	if err != nil {
		return fmt.Errorf("failed to delete script action: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("'%s' deleted.", scriptName)})
}
