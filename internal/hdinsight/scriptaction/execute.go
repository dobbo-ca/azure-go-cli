package scriptaction

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

func Execute(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, name, scriptURI string, roles []string, parameters string, persistOnSuccess, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	clusters, err := armhdinsight.NewClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create clusters client: %w", err)
	}

	rolePtrs := make([]*string, 0, len(roles))
	for _, r := range roles {
		rolePtrs = append(rolePtrs, to.Ptr(r))
	}

	action := &armhdinsight.RuntimeScriptAction{
		Name:  to.Ptr(name),
		URI:   to.Ptr(scriptURI),
		Roles: rolePtrs,
	}
	if parameters != "" {
		action.Parameters = to.Ptr(parameters)
	}

	params := armhdinsight.ExecuteScriptActionParameters{
		PersistOnSuccess: to.Ptr(persistOnSuccess),
		ScriptActions:    []*armhdinsight.RuntimeScriptAction{action},
	}

	fmt.Printf("Executing script action '%s' on cluster '%s'...\n", name, clusterName)
	poller, err := clusters.BeginExecuteScriptActions(ctx, resourceGroup, clusterName, params, nil)
	if err != nil {
		return fmt.Errorf("failed to begin execute script actions: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "script action execution started"})
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("execute script actions failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("script action '%s' executed.", name)})
}
