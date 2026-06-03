package pim

import (
	"fmt"
	"regexp"
	"strings"
)

// ScopeIndexEntry is one resolvable subscription scope built from the user's
// eligible PIM assignments cross-referenced with the local azureProfile cache.
type ScopeIndexEntry struct {
	ArmPath           string
	SubscriptionID    string
	SubscriptionName  string
	TenantID          string
	TenantDisplayName string
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ResolveScope expands a user-supplied scope into a full ARM path.
//
// Accepted forms (matched in order):
//  1. Full ARM path starting with "/subscriptions/" — returned verbatim.
//  2. Subscription UUID — expanded to "/subscriptions/<UUID>".
//  3. "tenant-name/subscription-name[/resource-group]" — looked up in the index.
//  4. Bare subscription name — accepted only if it matches exactly one entry.
//
// Ambiguous matches return an error listing the candidates.
func ResolveScope(input string, index []ScopeIndexEntry) (string, error) {
	if input == "" {
		return "", fmt.Errorf("scope is empty")
	}

	// Form 1: full ARM path.
	if strings.HasPrefix(input, "/subscriptions/") {
		return input, nil
	}

	// Form 2: bare UUID.
	if uuidRe.MatchString(input) {
		return "/subscriptions/" + input, nil
	}

	// Form 3: tenant/subscription[/rg].
	if strings.Contains(input, "/") {
		parts := strings.SplitN(input, "/", 3)
		tenantName, subName := parts[0], parts[1]
		var rgSuffix string
		if len(parts) == 3 && parts[2] != "" {
			rgSuffix = "/resourceGroups/" + parts[2]
		}
		for _, e := range index {
			if e.TenantDisplayName == tenantName && e.SubscriptionName == subName {
				return e.ArmPath + rgSuffix, nil
			}
		}
		return "", fmt.Errorf("no eligible scope matches %q", input)
	}

	// Form 4: bare subscription name — must be unambiguous.
	var matches []ScopeIndexEntry
	for _, e := range index {
		if e.SubscriptionName == input {
			matches = append(matches, e)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no eligible scope matches %q", input)
	case 1:
		return matches[0].ArmPath, nil
	default:
		var candidates []string
		for _, m := range matches {
			candidates = append(candidates, fmt.Sprintf("%s/%s", m.TenantDisplayName, m.SubscriptionName))
		}
		return "", fmt.Errorf("ambiguous scope %q; candidates: %s", input, strings.Join(candidates, ", "))
	}
}
