package managementpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, account, resourceGroup, policyFile string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(policyFile)
	if err != nil {
		return fmt.Errorf("failed to read policy file: %w", err)
	}
	var schema armstorage.ManagementPolicySchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return fmt.Errorf("failed to parse policy file: %w", err)
	}

	client, err := armstorage.NewManagementPoliciesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create management policies client: %w", err)
	}

	props := armstorage.ManagementPolicy{
		Properties: &armstorage.ManagementPolicyProperties{
			Policy: &schema,
		},
	}

	resp, err := client.CreateOrUpdate(ctx, resourceGroup, account, armstorage.ManagementPolicyNameDefault, props, nil)
	if err != nil {
		return fmt.Errorf("failed to create management policy: %w", err)
	}
	return output.PrintJSON(cmd, resp.ManagementPolicy)
}
