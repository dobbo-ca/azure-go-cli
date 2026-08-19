package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func securityGroupUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update name and/or description for an Azure DevOps group.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityGroupUpdateRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	cmd.Flags().String("id", "", "Descriptor of the group.")
	cmd.Flags().String("name", "", "New name for Azure DevOps group.")
	cmd.Flags().String("description", "", "New description for Azure DevOps group.")
	cmd.MarkFlagRequired("id")

	return cmd
}

func securityGroupUpdateRun(ctx context.Context, cmd *cobra.Command) error {
	// security_group.py:123-124: empty string is a valid explicit value for
	// either field, so presence is checked with Changed, not non-emptiness.
	nameSet := cmd.Flags().Changed("name")
	descSet := cmd.Flags().Changed("description")
	if !nameSet && !descSet {
		return fmt.Errorf("Either name or description argument must be provided.")
	}

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := securityNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")

	var patch []map[string]any
	if nameSet {
		patch = append(patch, map[string]any{"op": "replace", "path": "/displayName", "value": name})
	}
	if descSet {
		patch = append(patch, map[string]any{"op": "replace", "path": "/description", "value": description})
	}

	var group map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     "PATCH",
		Host:       "vssps",
		Path:       "Graph/Groups/" + url.PathEscape(id),
		APIVersion: "5.0-preview.1",
		Body:       patch,
		JSONPatch:  true,
	}, &group); err != nil {
		return fmt.Errorf("failed to update group: %w", err)
	}

	return ado.Print(cmd, group, securityGroupShowColumns...)
}
