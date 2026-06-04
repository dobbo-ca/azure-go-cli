package pim

import (
	"fmt"

	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

// SetDefaultSubscription mutates the profile in place: marks subID as default
// and clears IsDefault on every other entry. Errors if subID is not present.
// The caller is responsible for persisting the profile.
func SetDefaultSubscription(p *config.Profile, subID string) error {
	found := false
	for i := range p.Subscriptions {
		if p.Subscriptions[i].ID == subID {
			p.Subscriptions[i].IsDefault = true
			found = true
		} else {
			p.Subscriptions[i].IsDefault = false
		}
	}
	if !found {
		return fmt.Errorf("subscription %q not in local profile; run `az account list` against this tenant first", subID)
	}
	return nil
}
