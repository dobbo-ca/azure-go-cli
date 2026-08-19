package devops

import (
	"net/url"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newExtensionCommand returns the "az devops extension" command group:
// list, search, show, install, uninstall, enable, disable
// (dev/team/commands.py:140-147).
func newExtensionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extension",
		Short: "Manage extensions",
		Long:  "Manage Azure DevOps extensions",
	}

	cmd.AddCommand(newExtensionListCmd())
	cmd.AddCommand(newExtensionSearchCmd())
	cmd.AddCommand(newExtensionShowCmd())
	cmd.AddCommand(newExtensionInstallCmd())
	cmd.AddCommand(newExtensionUninstallCmd())
	cmd.AddCommand(newExtensionEnableCmd())
	cmd.AddCommand(newExtensionDisableCmd())

	return cmd
}

// extensionColumns is transform_extension_table_output /
// transform_extensions_table_output (dev/team/_format.py:28-50), shared by
// list/show/install/enable/disable. Note the literal trailing spaces in
// "Version " and "Last Updated " — not typos, keep them.
var extensionColumns = []ado.Column{
	{Header: "Publisher Id", Value: func(row map[string]any) string { return extensionTrim(devopsStr(row["publisherId"]), 10) }},
	{Header: "Extension Id", Value: func(row map[string]any) string { return extensionTrim(devopsStr(row["extensionId"]), 20) }},
	{Header: "Name", Value: func(row map[string]any) string { return extensionTrim(devopsStr(row["extensionName"]), 20) }},
	{Header: "Version ", Value: func(row map[string]any) string { return extensionTrim(devopsStr(row["version"]), 20) }},
	{Header: "Last Updated ", Value: func(row map[string]any) string { return extensionDateOnly(devopsStr(row["lastPublished"])) }},
	{Header: "States", Value: func(row map[string]any) string { return extensionTrim(extensionInstallStateFlags(row), 20) }},
	{Header: "Flags", Value: func(row map[string]any) string { return extensionTrim(devopsStr(row["flags"]), 20) }},
}

// extensionSearchColumns is transform_extension_search_results_table_output
// (dev/team/_format.py:12-25) — these are Field paths (JMESPath) straight
// into the raw Marketplace JSON, not InstalledExtension field names.
var extensionSearchColumns = []ado.Column{
	{Header: "Publisher Name", Field: "publisher.publisherName"},
	{Header: "Extension Name", Field: "extensionName"},
	{Header: "Name", Field: "displayName"},
}

// extensionNewClient is a test seam so extension_test.go can point commands
// at an httptest server without depending on real Azure DevOps auth (mirrors
// the getCredential var seam in ado/auth.go).
var extensionNewClient = ado.NewClient

// extensionAddIDFlags registers --publisher-id/--extension-id, required on
// show/install/uninstall/enable/disable (dev/team/arguments.py:157-165).
func extensionAddIDFlags(cmd *cobra.Command, publisherID, extensionID *string) {
	cmd.Flags().StringVar(publisherID, "publisher-id", "", "Publisher Id. This will map to publisher-name in the az devops extension search output.")
	cmd.Flags().StringVar(extensionID, "extension-id", "", "Extension Id. This will map to extension-name in the az devops extension search output.")
	cmd.MarkFlagRequired("publisher-id")
	cmd.MarkFlagRequired("extension-id")
}

// extensionByNamePath builds the
// extensionmanagement/installedextensionsbyname/{publisher}/{extension}
// route shared by get/install/uninstall_extension_by_name
// (extension_management_client.py:66-133).
func extensionByNamePath(publisherID, extensionID string) string {
	return "extensionmanagement/installedextensionsbyname/" + url.PathEscape(publisherID) + "/" + url.PathEscape(extensionID)
}

func extensionInstallStateFlags(row map[string]any) string {
	state, _ := row["installState"].(map[string]any)
	return devopsStr(state["flags"])
}

// extensionTrim ports dev/common/format.py's trim_for_display: falsy (empty)
// text passes through unchanged, otherwise truncate at max and append "...".
func extensionTrim(text string, max int) string {
	if text == "" {
		return text
	}
	if r := []rune(text); len(r) > max {
		return string(r[:max]) + "..."
	}
	return text
}

// extensionDateOnly ports date_time_to_only_date (dev/common/format.py:24-32):
// best-effort parse to a date; on failure return the original string
// unchanged, matching Python's broad except-and-return-input fallback.
func extensionDateOnly(s string) string {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return s
}

// extensionFlagsList splits an InstalledExtensionState.flags string like
// "eventCallbacksBypassed, disabled" into trimmed tokens, mirroring
// extension.py:180/187's naive state_from_service.split(',').
func extensionFlagsList(flags string) []string {
	parts := strings.Split(flags, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
