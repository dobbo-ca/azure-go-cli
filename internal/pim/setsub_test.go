package pim

import (
	"testing"

	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func TestSetDefaultSubscription_Marks(t *testing.T) {
	p := &config.Profile{
		Subscriptions: []config.Subscription{
			{ID: "sub-a", Name: "Acme Production", IsDefault: true},
			{ID: "sub-b", Name: "Acme Dev", IsDefault: false},
		},
	}
	if err := SetDefaultSubscription(p, "sub-b"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if p.Subscriptions[0].IsDefault {
		t.Error("sub-a should no longer be default")
	}
	if !p.Subscriptions[1].IsDefault {
		t.Error("sub-b should be default")
	}
}

func TestSetDefaultSubscription_NotInProfile(t *testing.T) {
	p := &config.Profile{
		Subscriptions: []config.Subscription{
			{ID: "sub-a", Name: "Acme Production", IsDefault: true},
		},
	}
	err := SetDefaultSubscription(p, "sub-unknown")
	if err == nil {
		t.Fatal("want error when subscription is not in profile")
	}
}
