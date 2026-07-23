package certificate

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
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

	client, err := azcertificates.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create certificate client: %w", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	resp, err := client.RestoreCertificate(ctx, azcertificates.RestoreCertificateParameters{CertificateBackup: data}, nil)
	if err != nil {
		return fmt.Errorf("failed to restore certificate: %w", err)
	}

	return output.PrintJSON(cmd, resp.Certificate)
}
