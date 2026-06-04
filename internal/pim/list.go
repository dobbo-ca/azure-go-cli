package pim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	pimvendor "github.com/cdobbyn/azure-go-cli/internal/pim/vendor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

// ListRow is one rendered table row, unified across resource and group types.
type ListRow struct {
	Type         string `json:"type"`
	Tenant       string `json:"tenant"`
	Subscription string `json:"subscription"`
	Name         string `json:"name"`
	Status       string `json:"status"`
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List eligible and active PIM assignments",
		RunE: func(cmd *cobra.Command, args []string) error {
			typeFilter, _ := cmd.Flags().GetString("type")
			outFmt, _ := cmd.Flags().GetString("output")
			return runList(cmd.Context(), typeFilter, outFmt, cmd.OutOrStdout())
		},
	}
	cmd.Flags().String("type", "", "filter by type: resource or group")
	cmd.Flags().String("output", "table", "output format: table or json")
	return cmd
}

func runList(ctx context.Context, typeFilter, outFmt string, w io.Writer) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return fmt.Errorf("get credential: %w", err)
	}
	ts := NewTokenSource(cred)
	client := pimvendor.AzureClient{ARMBaseURL: pimvendor.ARM_GLOBAL_BASE_URL}

	_ = ctx
	rows, err := collectRows(ts, client, typeFilter)
	if err != nil {
		return err
	}

	switch strings.ToLower(outFmt) {
	case "json":
		return RenderListJSON(w, rows)
	case "table", "":
		return RenderListTable(w, rows)
	default:
		return fmt.Errorf("unknown --output %q (use table or json)", outFmt)
	}
}

func collectRows(ts *TokenSource, client pimvendor.AzureClient, typeFilter string) ([]ListRow, error) {
	profile, _ := config.Load() // tolerate missing profile; tenant names degrade to UUIDs

	var rows []ListRow

	if typeFilter == "" || typeFilter == "resource" {
		armToken, err := ts.GetAccessToken("https://management.azure.com/.default")
		if err != nil {
			return nil, err
		}
		resp, err := client.GetEligibleResourceAssignments(armToken)
		if err != nil {
			return nil, err
		}
		for _, a := range resp.Value {
			if a.Properties == nil {
				continue
			}
			tenant, sub := lookupTenantAndSub(profile, a.Properties.Scope)
			var roleDisplayName string
			if a.Properties.ExpandedProperties != nil {
				roleDisplayName = displayNameOrEmpty(a.Properties.ExpandedProperties.RoleDefinition)
			}
			rows = append(rows, ListRow{
				Type:         "resource",
				Tenant:       tenant,
				Subscription: sub,
				Name:         roleDisplayName,
				Status:       formatStatus(a.Properties.Status, a.Properties.EndDateTime),
			})
		}
	}

	if typeFilter == "" || typeFilter == "group" {
		graphToken, err := ts.GetAccessToken("https://graph.microsoft.com/.default")
		if err != nil {
			return nil, err
		}
		info, err := pimvendor.GetUserInfo(graphToken)
		if err != nil {
			return nil, err
		}
		resp, err := client.GetEligibleGovernanceRoleAssignments(pimvendor.ROLE_TYPE_AAD_GROUPS, info.ObjectId, graphToken)
		if err != nil {
			return nil, err
		}
		for _, a := range resp.Value {
			var tenantID, name string
			if a.RoleDefinition != nil {
				name = a.RoleDefinition.DisplayName
				if a.RoleDefinition.Resource != nil {
					tenantID = a.RoleDefinition.Resource.Id
				}
			}
			rows = append(rows, ListRow{
				Type:         "group",
				Tenant:       resolveTenantName(profile, tenantID),
				Subscription: "—",
				Name:         name,
				Status:       formatStatus(a.AssignmentState, ""), // eligible groups have no expiry until activated
			})
		}
	}

	return rows, nil
}

// formatStatus turns ("Eligible", "...") into "Eligible" and
// ("Active", "2026-05-14T15:42:00Z") into "Active (expires 15:42 UTC)".
func formatStatus(state, endRFC3339 string) string {
	if !strings.EqualFold(state, "Active") || endRFC3339 == "" {
		return state
	}
	if len(endRFC3339) >= 16 {
		return fmt.Sprintf("Active (expires %s UTC)", endRFC3339[11:16])
	}
	return state
}

func displayNameOrEmpty(p *pimvendor.ResourceExpandedProperty) string {
	if p == nil {
		return ""
	}
	return p.DisplayName
}

func lookupTenantAndSub(p *config.Profile, armScope string) (tenant, sub string) {
	subID := extractSubscriptionID(armScope)
	if subID == "" {
		return "", ""
	}
	if p == nil {
		return "", subID
	}
	for _, s := range p.Subscriptions {
		if s.ID == subID {
			return s.TenantID, s.Name
		}
	}
	return "", subID
}

func resolveTenantName(p *config.Profile, tenantID string) string {
	if p == nil {
		return tenantID
	}
	for _, s := range p.Subscriptions {
		if s.TenantID == tenantID {
			return s.TenantID
		}
	}
	return tenantID
}

func extractSubscriptionID(armScope string) string {
	const prefix = "/subscriptions/"
	if !strings.HasPrefix(armScope, prefix) {
		return ""
	}
	rest := armScope[len(prefix):]
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[:i]
	}
	return rest
}

// RenderListTable writes a tab-aligned human table.
func RenderListTable(w io.Writer, rows []ListRow) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "TYPE\tTENANT\tSUBSCRIPTION\tNAME\tSTATUS")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Type, r.Tenant, r.Subscription, r.Name, r.Status)
	}
	return tw.Flush()
}

// RenderListJSON writes the rows as a JSON array. An empty input is emitted
// as `[]`, never `null`, so downstream `jq`/scripts can consume it cleanly.
func RenderListJSON(w io.Writer, rows []ListRow) error {
	if rows == nil {
		rows = []ListRow{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}
