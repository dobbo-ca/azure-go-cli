package secret

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Restore(ctx context.Context, cmd *cobra.Command, vaultName, file string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)

	client, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create secrets client: %w", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	resp, err := client.RestoreSecret(ctx, azsecrets.RestoreSecretParameters{SecretBackup: data}, nil)
	if err != nil {
		return fmt.Errorf("failed to restore secret: %w", err)
	}

	return output.PrintJSON(cmd, resp.Secret)
}
