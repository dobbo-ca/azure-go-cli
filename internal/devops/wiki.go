package devops

import (
	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newWikiCommand returns the "az devops wiki" command group (dev/team/wiki.py,
// registered dev/team/commands.py:173-178), including the nested
// "az devops wiki page" subgroup (commands.py:180-184).
func newWikiCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki",
		Short: "Manage wikis",
		Long:  "Manage Azure DevOps wikis",
	}

	cmd.AddCommand(wikiCreateCmd())
	cmd.AddCommand(wikiListCmd())
	cmd.AddCommand(wikiShowCmd())
	cmd.AddCommand(wikiDeleteCmd())
	cmd.AddCommand(wikiPageCmd())

	return cmd
}

// wikiColumns is _transform_wiki_row (dev/team/_format.py:293-297), shared by
// create/list/show/delete.
var wikiColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Type", Field: "type"},
}

// devopsStr reads a map[string]any value expected to be a string, tolerating
// a missing/non-string value as "". Shared by the wiki/team/extension table
// column renderers.
func devopsStr(v any) string {
	s, _ := v.(string)
	return s
}
