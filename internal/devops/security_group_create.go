package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func securityGroupCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Azure DevOps group, or materialize an existing AAD or AD group.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityGroupCreateRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.Flags().String("name", "", "Name of Azure DevOps group.")
	cmd.Flags().String("description", "", "Description of Azure DevOps group.")
	cmd.Flags().String("origin-id", "", "Create new group using the OriginID as a reference to an existing group from an external AD or AAD backed provider.")
	cmd.Flags().String("email-id", "", "Create new group using the mail address as a reference to an existing group from an external AD or AAD backed provider.")
	cmd.Flags().String("groups", "", "A comma separated list of descriptors referencing groups you want the newly created group to join.")
	cmd.Flags().String("scope", "project", "Create group at project or organization level. Allowed values: project, organization.")

	return cmd
}

func securityGroupCreateRun(ctx context.Context, cmd *cobra.Command) error {
	scope, _ := cmd.Flags().GetString("scope")
	if scope != "project" && scope != "organization" {
		return fmt.Errorf("--scope must be one of project, organization")
	}

	// security_group.py:75-80: same scope branch as `security group list` —
	// scope=='organization' resolves the org only (resolve_instance),
	// leaving project exactly the raw --project value with no
	// git-detected/config-default fallback.
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

	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	originID, _ := cmd.Flags().GetString("origin-id")
	emailID, _ := cmd.Flags().GetString("email-id")
	nameSet := cmd.Flags().Changed("name")
	originSet := cmd.Flags().Changed("origin-id")
	emailSet := cmd.Flags().Changed("email-id")

	// security_group.py:82-89: a straight if/elif/else, replicated exactly
	// (including that all-unset falls into the else and errors).
	var body map[string]any
	switch {
	case nameSet && !originSet && !emailSet:
		body = map[string]any{"displayName": name}
		if cmd.Flags().Changed("description") {
			body["description"] = description
		}
	case originSet && !emailSet && !nameSet:
		body = map[string]any{"originId": originID}
	case emailSet && !nameSet && !originSet:
		body = map[string]any{"mailAddress": emailID}
	default:
		return fmt.Errorf("Provide exactly one argument out of name, origin-id or email-id.")
	}

	q := url.Values{}
	if dctx.Project != "" {
		scopeDescriptor, err := securityScopeDescriptor(ctx, client, dctx.Project)
		if err != nil {
			return fmt.Errorf("failed to resolve project scope: %w", err)
		}
		q.Set("scopeDescriptor", scopeDescriptor)
	}
	if groups, _ := cmd.Flags().GetString("groups"); groups != "" {
		q.Set("groupDescriptors", groups)
	}

	var group map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     "POST",
		Host:       "vssps",
		Path:       "Graph/Groups",
		APIVersion: "5.0-preview.1",
		Query:      q,
		Body:       body,
	}, &group); err != nil {
		return fmt.Errorf("failed to create group: %w", err)
	}

	return ado.Print(cmd, group, securityGroupShowColumns...)
}
