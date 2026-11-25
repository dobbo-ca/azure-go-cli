package support

import (
	"context"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

type resourceRef struct{ ID, Name, Type string }

// resourceTypeFilter builds an ARM $filter matching any of the resource types.
func resourceTypeFilter(types []string) string {
	parts := make([]string, len(types))
	for i, t := range types {
		parts[i] = "resourceType eq '" + strings.ReplaceAll(t, "'", "''") + "'"
	}
	return strings.Join(parts, " or ")
}

// listResources lists the subscription's resources of the given types, sorted by name.
func listResources(ctx context.Context, s Scope, types []string) ([]resourceRef, error) {
	if len(types) == 0 {
		return nil, nil // an empty $filter would be sent to ARM
	}
	cred, err := credentialFor(s.TenantID)
	if err != nil {
		return nil, err
	}
	client, err := armresources.NewClient(s.SubscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	f := resourceTypeFilter(types)
	pager := client.NewListPager(&armresources.ClientListOptions{Filter: &f})
	var out []resourceRef
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range page.Value {
			out = append(out, resourceRef{ID: deref(r.ID), Name: deref(r.Name), Type: deref(r.Type)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
