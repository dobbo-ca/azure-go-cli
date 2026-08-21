package certificate

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// Create makes a certificate and returns its pending operation. create_certificate
// (custom.py:1737) returns the operation immediately when the issuer is
// Unknown, and otherwise waits for the operation to finish first.
func Create(ctx context.Context, cmd *cobra.Command, vaultName, name, policyValue string, validity int32, disabled bool, tags []string) error {
	policy, err := ParsePolicy(policyValue, validity)
	if err != nil {
		return err
	}
	if policy == nil {
		return fmt.Errorf("--policy is required")
	}
	client, err := certificateClient(vaultName)
	if err != nil {
		return err
	}
	params := azcertificates.CreateCertificateParameters{
		CertificatePolicy:     policy,
		CertificateAttributes: &azcertificates.CertificateAttributes{Enabled: to.Ptr(!disabled)},
		Tags:                  parseCertTags(tags),
	}
	if _, err := client.CreateCertificate(ctx, name, params, nil); err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}
	resp, err := client.GetCertificateOperation(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get the certificate operation: %w", err)
	}
	return output.PrintJSON(cmd, resp.CertificateOperation)
}
