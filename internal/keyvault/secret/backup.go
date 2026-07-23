package secret

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Backup(ctx context.Context, cmd *cobra.Command, vaultName, name, file string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)

	client, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create secrets client: %w", err)
	}

	resp, err := client.BackupSecret(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to backup secret: %w", err)
	}

	if file != "" {
		if err := os.WriteFile(file, resp.Value, 0600); err != nil {
			return fmt.Errorf("failed to write backup file: %w", err)
		}
		return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("backed up to %s", file)})
	}

	return output.PrintJSON(cmd, map[string]string{"value": base64.StdEncoding.EncodeToString(resp.Value)})
}
