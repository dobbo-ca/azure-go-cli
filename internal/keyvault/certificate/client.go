package certificate

import (
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

// certificateClient opens a data-plane client for a vault.
func certificateClient(vaultName string) (*azcertificates.Client, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)
	client, err := azcertificates.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate client: %w", err)
	}
	return client, nil
}

// parseCertTags turns "key=value" pairs into a tag map.
func parseCertTags(pairs []string) map[string]*string {
	if len(pairs) == 0 {
		return nil
	}
	tags := make(map[string]*string, len(pairs))
	for _, pair := range pairs {
		key, value, _ := strings.Cut(pair, "=")
		tags[key] = to.Ptr(value)
	}
	return tags
}
