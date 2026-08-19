package devops

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func wikiListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all the wikis in a project or organization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return wikiRunList(context.Background(), cmd)
		},
	}

	cmd.Flags().String("scope", "project", "List the wikis at project or organization level. Allowed values: project, organization.")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func wikiRunList(ctx context.Context, cmd *cobra.Command) error {
	scope, _ := cmd.Flags().GetString("scope")
	if scope == "" {
		scope = "project"
	}
	if scope != "project" && scope != "organization" {
		return fmt.Errorf("--scope must be one of: project, organization")
	}

	var dctx ado.Context
	var err error
	if scope == "project" {
		dctx, err = ado.ResolveProject(cmd)
	} else {
		dctx, err = ado.Resolve(cmd)
		// wiki.py:88-96: the organization-scope branch only resolves the
		// organization (resolve_instance) — project stays exactly the raw
		// --project value passed in, with no git-detected/config-default
		// fallback (unlike ado.Resolve's project, which git-detection can
		// still populate as a side effect).
		dctx.Project, _ = cmd.Flags().GetString("project")
	}
	if err != nil {
		return err
	}

	return wikiList(ctx, cmd, dctx, scope)
}

// wikiList does the actual client call, split out from wikiRunList so tests
// can supply a dctx pointing at an httptest server without going through org
// validation.
func wikiList(ctx context.Context, cmd *cobra.Command, dctx ado.Context, scope string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// wiki_client.py:323-325 (get_all_wikis): the project route value is
	// added whenever project is non-empty, regardless of scope — so an
	// explicit --project alongside --scope organization still scopes the
	// request (wiki.py:96 passes project through unconditionally).
	req := ado.Request{Path: "wiki/wikis", APIVersion: "5.0"}
	if dctx.Project != "" {
		req.Scope = dctx.Project
	}

	var wikis []map[string]any
	if err := client.List(ctx, req, &wikis); err != nil {
		return fmt.Errorf("failed to list wikis: %w", err)
	}

	// transform_wikis_table_output sorts by name.lower() (_get_wiki_key,
	// dev/team/_format.py:279-283,373-374), but commands.py:175 wires it
	// only as this command's table_transformer — knack applies that solely
	// for -o table with no --query; JSON/tsv keep the server's order.
	if ado.TableMode(cmd) {
		sort.Slice(wikis, func(i, j int) bool {
			return strings.ToLower(devopsStr(wikis[i]["name"])) < strings.ToLower(devopsStr(wikis[j]["name"]))
		})
	}

	return ado.Print(cmd, wikis, wikiColumns...)
}
