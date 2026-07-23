package certificate

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
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

	client, err := azcertificates.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create certificate client: %w", err)
	}

	var enabled *bool
	if cmd.Flags().Changed("enabled") {
		b, _ := cmd.Flags().GetBool("enabled")
		enabled = &b
	}

	resp, err := client.UpdateCertificate(ctx, name, version, azcertificates.UpdateCertificateParameters{
		CertificateAttributes: &azcertificates.CertificateAttributes{Enabled: enabled},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to update certificate: %w", err)
	}

	return output.PrintJSON(cmd, resp.Certificate)
}
