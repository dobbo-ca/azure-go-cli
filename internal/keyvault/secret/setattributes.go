package secret

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func SetAttributes(ctx context.Context, cmd *cobra.Command, vaultName, name, version string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)

	client, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create secrets client: %w", err)
	}

	var enabled *bool
	if cmd.Flags().Changed("enabled") {
		b, _ := cmd.Flags().GetBool("enabled")
		enabled = &b
	}

	params := azsecrets.UpdateSecretPropertiesParameters{
		SecretAttributes: &azsecrets.SecretAttributes{Enabled: enabled},
	}

	resp, err := client.UpdateSecretProperties(ctx, name, version, params, nil)
	if err != nil {
		return fmt.Errorf("failed to update secret properties: %w", err)
	}

	return output.PrintJSON(cmd, resp.Secret)
}
