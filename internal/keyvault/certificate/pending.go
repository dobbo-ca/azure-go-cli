package certificate

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// PendingShow reports the state of a pending certificate operation.
func PendingShow(ctx context.Context, cmd *cobra.Command, vaultName, name string) error {
	client, err := certificateClient(vaultName)
	if err != nil {
		return err
	}
	resp, err := client.GetCertificateOperation(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get the certificate operation: %w", err)
	}
	return output.PrintJSON(cmd, resp.CertificateOperation)
}

// PendingDelete cancels a pending certificate operation.
func PendingDelete(ctx context.Context, cmd *cobra.Command, vaultName, name string) error {
	client, err := certificateClient(vaultName)
	if err != nil {
		return err
	}
	resp, err := client.DeleteCertificateOperation(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to delete the certificate operation: %w", err)
	}
	return output.PrintJSON(cmd, resp.CertificateOperation)
}

// PendingMerge merges a signed certificate chain into a pending certificate.
func PendingMerge(ctx context.Context, cmd *cobra.Command, vaultName, name, file string, disabled bool, tags []string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("unable to load certificate file '%s': %w", file, err)
	}
	chain, err := splitCertificateChain(data)
	if err != nil {
		return err
	}
	client, err := certificateClient(vaultName)
	if err != nil {
		return err
	}
	params := azcertificates.MergeCertificateParameters{
		X509Certificates:      chain,
		CertificateAttributes: &azcertificates.CertificateAttributes{Enabled: to.Ptr(!disabled)},
		Tags:                  parseCertTags(tags),
	}
	resp, err := client.MergeCertificate(ctx, name, params, nil)
	if err != nil {
		return fmt.Errorf("failed to merge certificate: %w", err)
	}
	return output.PrintJSON(cmd, resp.Certificate)
}
