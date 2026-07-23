package certificate

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Recover(ctx context.Context, cmd *cobra.Command, vaultName, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)

	client, err := azcertificates.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create certificate client: %w", err)
	}

	resp, err := client.RecoverDeletedCertificate(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to recover certificate: %w", err)
	}

	return output.PrintJSON(cmd, resp.Certificate)
}
