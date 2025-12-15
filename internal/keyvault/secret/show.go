package secret

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, vaultName, name string, showValue bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	// Key Vault URL format: https://{vault-name}.vault.azure.net/
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)

	client, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create secrets client: %w", err)
	}

	result, err := client.GetSecret(ctx, name, "", nil)
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// If not showing value, clear it from the response
	if !showValue && result.Value != nil {
		result.Value = nil
	}

	return output.PrintJSON(cmd, result.Secret)
}
