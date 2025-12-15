package secret

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Set(ctx context.Context, cmd *cobra.Command, vaultName, name, value string, tags map[string]string) error {
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

	// Convert tags to Azure format
	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	parameters := azsecrets.SetSecretParameters{
		Value: to.Ptr(value),
		Tags:  azureTags,
	}

	result, err := client.SetSecret(ctx, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to set secret: %w", err)
	}

	return output.PrintJSON(cmd, result.Secret)
}
