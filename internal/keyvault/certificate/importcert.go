package certificate

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// pemCertificate matches one PEM certificate block, as
// validate_x509_certificate_chain does (_validators.py:476).
var pemCertificate = regexp.MustCompile(`-----BEGIN CERTIFICATE-----([^-]+)-----END CERTIFICATE-----`)

// splitCertificateChain reads every certificate in a PEM chain as raw DER.
func splitCertificateChain(data []byte) ([][]byte, error) {
	var chain [][]byte
	for _, match := range pemCertificate.FindAllStringSubmatch(string(data), -1) {
		body := strings.NewReplacer("\n", "", "\r", "", " ", "").Replace(match[1])
		der, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return nil, fmt.Errorf("failed to decode the certificate: %w", err)
		}
		chain = append(chain, der)
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("no certificate found in the file")
	}
	return chain, nil
}

// Import uploads a PKCS12 or PEM file holding a certificate and its private
// key (custom.py, import_certificate on the SDK client).
func Import(ctx context.Context, cmd *cobra.Command, vaultName, name, file, password, policyValue string, disabled bool, tags []string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("unable to load certificate file '%s': %w", file, err)
	}
	policy, err := ParsePolicy(policyValue, 0)
	if err != nil {
		return err
	}
	// process_certificate_policy (_validators.py:893) fills in the content
	// type from the file itself when the policy leaves it out.
	if policy == nil {
		policy = &azcertificates.CertificatePolicy{}
	}
	if policy.SecretProperties == nil || policy.SecretProperties.ContentType == nil {
		contentType := "application/x-pkcs12"
		if pemCertificate.Match(data) {
			contentType = "application/x-pem-file"
		}
		policy.SecretProperties = &azcertificates.SecretProperties{ContentType: to.Ptr(contentType)}
	}

	client, err := certificateClient(vaultName)
	if err != nil {
		return err
	}
	params := azcertificates.ImportCertificateParameters{
		Base64EncodedCertificate: to.Ptr(base64.StdEncoding.EncodeToString(data)),
		CertificatePolicy:        policy,
		CertificateAttributes:    &azcertificates.CertificateAttributes{Enabled: to.Ptr(!disabled)},
		Tags:                     parseCertTags(tags),
	}
	if password != "" {
		params.Password = to.Ptr(password)
	}
	resp, err := client.ImportCertificate(ctx, name, params, nil)
	if err != nil {
		return fmt.Errorf("failed to import certificate: %w", err)
	}
	return output.PrintJSON(cmd, resp.Certificate)
}
