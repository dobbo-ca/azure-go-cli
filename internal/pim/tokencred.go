package pim

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// TokenSource adapts an azcore.TokenCredential to the vendored PIM client's
// "GetAccessToken(scope) (string, error)" expectation. It does not satisfy the
// vendored Client interface itself — the caller composes a real AzureClient
// from the vendored package and supplies tokens by calling
// TokenSource.GetAccessToken for each scope.
type TokenSource struct {
	cred azcore.TokenCredential
}

func NewTokenSource(cred azcore.TokenCredential) *TokenSource {
	return &TokenSource{cred: cred}
}

// GetAccessToken acquires a bearer token for the given scope. The scope must
// end with "/.default" (e.g. "https://management.azure.com/.default").
func (t *TokenSource) GetAccessToken(scope string) (string, error) {
	tok, err := t.cred.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{scope},
	})
	if err != nil {
		return "", fmt.Errorf("acquire PIM token for %s: %w", scope, err)
	}
	return tok.Token, nil
}
