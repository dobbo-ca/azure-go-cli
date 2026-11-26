package support

import (
	"github.com/spf13/cobra"

	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

// Scope says which tenant, and which subscription if any, a client talks to.
type Scope struct{ TenantID, TenantName, SubscriptionID, SubscriptionName string }

// TenantLevel reports whether the scope has no subscription.
func (s Scope) TenantLevel() bool { return s.SubscriptionID == "" }

// Label is the header text for the scope.
func (s Scope) Label() string {
	tenant := s.TenantName
	if tenant == "" {
		tenant = s.TenantID
	}
	if s.TenantLevel() {
		return tenant + " · tenant-level (no subscription)"
	}
	return tenant + " · " + s.SubscriptionName + " (" + s.SubscriptionID + ")"
}

// flagScope resolves the scope from --subscription or the default subscription.
// The tenant is empty when the subscription is not in the profile.
func flagScope(cmd *cobra.Command) (Scope, error) {
	subFlag, _ := cmd.Flags().GetString("subscription")
	sub, err := config.GetSubscription(subFlag)
	if err != nil {
		return Scope{}, err
	}
	s := Scope{SubscriptionID: sub, SubscriptionName: sub}
	if tenant, err := config.GetTenantID(sub); err == nil {
		s.TenantID = tenant
	}
	if profile, err := config.Load(); err == nil {
		for _, p := range profile.Subscriptions {
			if p.ID == sub {
				s.SubscriptionName = p.Name
			}
		}
	}
	return s, nil
}

// commandClient builds the client for a command: tenant-level when asked, else
// subscription-scoped. sessionSub is the default or --subscription ID, also at
// tenant level, where it is only used to validate --technical-resource.
func commandClient(cmd *cobra.Command, tenantLevel bool) (c Client, sessionSub string, err error) {
	s, err := flagScope(cmd)
	if err != nil {
		return nil, "", err
	}
	sessionSub = s.SubscriptionID
	if tenantLevel {
		s.SubscriptionID, s.SubscriptionName = "", ""
	}
	c, err = NewClient(s)
	return c, sessionSub, err
}

// commandClientFn is the test seam; every command calls it.
var commandClientFn = commandClient
