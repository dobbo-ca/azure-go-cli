package managementgroup

import (
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/managementgroups/armmanagementgroups"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

// MGInfo is the trimmed, script-friendly view of a management group.
type MGInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	TenantID    string `json:"tenantId,omitempty"`
	Type        string `json:"type,omitempty"`
	ID          string `json:"id"`
}

const mgProviderPrefix = "/providers/Microsoft.Management/managementGroups/"

// normalizeParentID converts a bare management-group name into its fully
// qualified parent ID. A value that already looks like a full resource ID is
// returned unchanged; an empty value yields an empty string.
func normalizeParentID(parent string) string {
	parent = strings.TrimSpace(parent)
	if parent == "" {
		return ""
	}
	if strings.HasPrefix(parent, "/providers/") {
		return parent
	}
	return mgProviderPrefix + parent
}

// newClient returns the tenant-scoped management-groups client with cached creds.
func newClient() (*armmanagementgroups.Client, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}
	return armmanagementgroups.NewClient(cred, nil)
}

func groupInfo(g *armmanagementgroups.ManagementGroup) MGInfo {
	info := MGInfo{
		Name: azure.GetStringValue(g.Name),
		Type: azure.GetStringValue(g.Type),
		ID:   azure.GetStringValue(g.ID),
	}
	if g.Properties != nil {
		info.DisplayName = azure.GetStringValue(g.Properties.DisplayName)
		info.TenantID = azure.GetStringValue(g.Properties.TenantID)
	}
	return info
}

func listItemInfo(g *armmanagementgroups.ManagementGroupInfo) MGInfo {
	info := MGInfo{
		Name: azure.GetStringValue(g.Name),
		Type: azure.GetStringValue(g.Type),
		ID:   azure.GetStringValue(g.ID),
	}
	if g.Properties != nil {
		info.DisplayName = azure.GetStringValue(g.Properties.DisplayName)
		info.TenantID = azure.GetStringValue(g.Properties.TenantID)
	}
	return info
}
