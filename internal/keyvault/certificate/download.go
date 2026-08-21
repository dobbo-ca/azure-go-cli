package certificate

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Download writes the public certificate, as download_certificate does
// (custom.py:1761).
func Download(ctx context.Context, _ *cobra.Command, vaultName, name, version, file, encoding string) error {
	if _, err := os.Stat(file); err == nil {
		return fmt.Errorf("file or directory named '%s' already exists", file)
	}
	client, err := certificateClient(vaultName)
	if err != nil {
		return err
	}
	resp, err := client.GetCertificate(ctx, name, version, nil)
	if err != nil {
		return fmt.Errorf("failed to get certificate: %w", err)
	}
	body := resp.CER
	if strings.ToUpper(encoding) != "DER" {
		body = encodeCertificatePEM(resp.CER)
	}
	if err := os.WriteFile(file, body, 0o600); err != nil {
		return fmt.Errorf("failed to write '%s': %w", file, err)
	}
	return nil
}

// encodeCertificatePEM wraps DER bytes the way base64.encodebytes does, in
// 76 character lines (custom.py:1770).
func encodeCertificatePEM(der []byte) []byte {
	encoded := base64.StdEncoding.EncodeToString(der)
	var body strings.Builder
	body.WriteString("-----BEGIN CERTIFICATE-----\n")
	for len(encoded) > 76 {
		body.WriteString(encoded[:76])
		body.WriteString("\n")
		encoded = encoded[76:]
	}
	if encoded != "" {
		body.WriteString(encoded)
		body.WriteString("\n")
	}
	body.WriteString("-----END CERTIFICATE-----\n")
	return []byte(body.String())
}
