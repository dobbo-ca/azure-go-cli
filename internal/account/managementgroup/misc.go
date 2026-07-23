package managementgroup

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/managementgroups/armmanagementgroups"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NameAvailability is the trimmed view of a check-name-availability result.
type NameAvailability struct {
	NameAvailable bool   `json:"nameAvailable"`
	Reason        string `json:"reason,omitempty"`
	Message       string `json:"message,omitempty"`
}

// EntityOut is the trimmed view of a management-group entity.
type EntityOut struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type,omitempty"`
	ID          string `json:"id"`
}

// CheckNameAvailability reports whether a management group name can be used.
func CheckNameAvailability(ctx context.Context, cmd *cobra.Command, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	apiClient, err := armmanagementgroups.NewAPIClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create management-groups API client: %w", err)
	}

	resp, err := apiClient.CheckNameAvailability(ctx, armmanagementgroups.CheckNameAvailabilityRequest{
		Name: to.Ptr(name),
		Type: to.Ptr("Microsoft.Management/managementGroups"),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to check name availability for %q: %w", name, err)
	}

	out := NameAvailability{Message: azure.GetStringValue(resp.Message)}
	if resp.NameAvailable != nil {
		out.NameAvailable = *resp.NameAvailable
	}
	if resp.Reason != nil {
		out.Reason = string(*resp.Reason)
	}

	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, out, format)
}

// ListEntities lists all entities (management groups and subscriptions) in the tenant.
func ListEntities(ctx context.Context, cmd *cobra.Command) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	entClient, err := armmanagementgroups.NewEntitiesClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create entities client: %w", err)
	}

	entities := []EntityOut{}
	pager := entClient.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list entities: %w", err)
		}
		for _, e := range page.Value {
			out := EntityOut{
				Name: azure.GetStringValue(e.Name),
				Type: azure.GetStringValue(e.Type),
				ID:   azure.GetStringValue(e.ID),
			}
			if e.Properties != nil {
				out.DisplayName = azure.GetStringValue(e.Properties.DisplayName)
			}
			entities = append(entities, out)
		}
	}

	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, entities, format)
}

// ShowHierarchySettings shows the hierarchy settings for a management group.
func ShowHierarchySettings(ctx context.Context, cmd *cobra.Command, name string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	hsClient, err := armmanagementgroups.NewHierarchySettingsClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create hierarchy-settings client: %w", err)
	}

	resp, err := hsClient.Get(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get hierarchy settings for %q: %w", name, err)
	}

	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, resp.HierarchySettings, format)
}
