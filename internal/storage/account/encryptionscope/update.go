package encryptionscope

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, account, resourceGroup, name, state string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armstorage.NewEncryptionScopesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create encryption scopes client: %w", err)
	}

	scope := armstorage.EncryptionScope{
		EncryptionScopeProperties: &armstorage.EncryptionScopeProperties{},
	}
	if state != "" {
		scope.EncryptionScopeProperties.State = to.Ptr(armstorage.EncryptionScopeState(state))
	}

	resp, err := client.Patch(ctx, resourceGroup, account, name, scope, nil)
	if err != nil {
		return fmt.Errorf("failed to update encryption scope: %w", err)
	}
	return output.PrintJSON(cmd, resp.EncryptionScope)
}
