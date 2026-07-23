package credentials

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, username, password string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armhdinsight.NewClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create clusters client: %w", err)
	}

	params := armhdinsight.UpdateGatewaySettingsParameters{
		IsCredentialEnabled: to.Ptr(true),
		UserName:            to.Ptr(username),
		Password:            to.Ptr(password),
	}

	fmt.Printf("Updating gateway credentials for '%s'...\n", clusterName)

	poller, err := client.BeginUpdateGatewaySettings(ctx, resourceGroup, clusterName, params, nil)
	if err != nil {
		return fmt.Errorf("failed to begin gateway settings update: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "gateway credentials update started"})
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("gateway settings update failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("gateway credentials updated for '%s'.", clusterName)})
}
