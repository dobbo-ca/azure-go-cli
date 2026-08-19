package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

var securityGroupListColumns = []ado.Column{
	{Header: "Name", Field: "principalName"},
	{Header: "Descriptor", Field: "descriptor"},
}

func securityGroupListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all the groups in a project or organization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityGroupListRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.Flags().String("scope", "project", "List groups for a particular project or the whole organization. Allowed values: project, organization.")
	cmd.Flags().String("continuation-token", "", "If there are more results than can be returned in a single page, this is the continuation token to resume from.")
	cmd.Flags().String("subject-types", "", "A comma separated list of user subject subtypes to reduce the retrieved results.")

	return cmd
}

func securityGroupListRun(ctx context.Context, cmd *cobra.Command) error {
	scope, _ := cmd.Flags().GetString("scope")
	if scope != "project" && scope != "organization" {
		return fmt.Errorf("--scope must be one of project, organization")
	}

	// security_group.py:37-42: project resolution (hence project-required)
	// only happens for scope=='project'; scope=='organization' resolves the
	// org only (resolve_instance), leaving project exactly the raw
	// --project value passed in — no git-detected/config-default fallback,
	// unlike ado.Resolve's project, which git-detection can still populate
	// as a side effect. An explicit --project still gets scoped below.
	var dctx ado.Context
	var err error
	if scope == "project" {
		dctx, err = ado.ResolveProject(cmd)
	} else {
		dctx, err = ado.Resolve(cmd)
		dctx.Project, _ = cmd.Flags().GetString("project")
	}
	if err != nil {
		return err
	}

	client, err := securityNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	q := url.Values{}
	if dctx.Project != "" {
		scopeDescriptor, err := securityScopeDescriptor(ctx, client, dctx.Project)
		if err != nil {
			return fmt.Errorf("failed to resolve project scope: %w", err)
		}
		q.Set("scopeDescriptor", scopeDescriptor)
	}
	if subjectTypes, _ := cmd.Flags().GetString("subject-types"); subjectTypes != "" {
		q.Set("subjectTypes", subjectTypes)
	}
	if token, _ := cmd.Flags().GetString("continuation-token"); token != "" {
		q.Set("continuationToken", token)
	}

	var groups []map[string]any
	// ado.Client.List follows X-MS-ContinuationToken to exhaustion, which
	// makes Python's "Showing only 500 groups..." stdout notice
	// (dev/team/_format.py:124-127) moot — see foundation-spec.md §6.
	if err := client.List(ctx, ado.Request{
		Host:       "vssps",
		Path:       "Graph/Groups",
		APIVersion: "5.0-preview.1",
		Query:      q,
	}, &groups); err != nil {
		return fmt.Errorf("failed to list groups: %w", err)
	}

	// graph_client.py:113-116: list_groups returns PagedGraphGroups
	// (graphGroups/continuationToken, models.py:380-392), not a bare array
	// — a bare array here made every --query "graphGroups[?...]" break
	// (also asserted by the extension's own integration test). No
	// DEVIATION comment needed for continuationToken: unlike core_invoke.go
	// et al, ado.Client.List already follows the header to exhaustion
	// (foundation-spec.md §6), so there genuinely is no token left by the
	// time this returns.
	if ado.TableMode(cmd) {
		return ado.Print(cmd, groups, securityGroupListColumns...)
	}
	return ado.Print(cmd, map[string]any{"graphGroups": groups, "continuationToken": nil})
}
