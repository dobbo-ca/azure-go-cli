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

func Download(ctx context.Context, cmd *cobra.Command, vaultName, name, file string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)

	client, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create secrets client: %w", err)
	}

	resp, err := client.GetSecret(ctx, name, "", nil)
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	if resp.Value == nil {
		return fmt.Errorf("secret has no value")
	}

	if err := os.WriteFile(file, []byte(*resp.Value), 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("downloaded to %s", file)})
}
