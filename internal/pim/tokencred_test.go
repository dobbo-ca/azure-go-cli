package pim

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type fakeCred struct {
	token string
	err   error
	calls []policy.TokenRequestOptions
}

func (f *fakeCred) GetToken(_ context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	f.calls = append(f.calls, opts)
	if f.err != nil {
		return azcore.AccessToken{}, f.err
	}
	return azcore.AccessToken{Token: f.token}, nil
}

func TestTokenSource_ReturnsToken(t *testing.T) {
	fc := &fakeCred{token: "abc"}
	ts := NewTokenSource(fc)
	got, err := ts.GetAccessToken("https://management.azure.com/.default")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != "abc" {
		t.Fatalf("got %q want abc", got)
	}
	if len(fc.calls) != 1 || fc.calls[0].Scopes[0] != "https://management.azure.com/.default" {
		t.Fatalf("expected single call with scope; got %+v", fc.calls)
	}
}

func TestTokenSource_PropagatesError(t *testing.T) {
	fc := &fakeCred{err: errors.New("boom")}
	ts := NewTokenSource(fc)
	_, err := ts.GetAccessToken("https://example/.default")
	if err == nil || err.Error() == "" {
		t.Fatalf("expected error to propagate; got %v", err)
	}
}
