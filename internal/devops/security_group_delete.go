package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func securityGroupDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an Azure DevOps group.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return securityGroupDeleteRun(context.Background(), cmd)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)
	cmd.Flags().String("id", "", "Descriptor of the group.")
	cmd.MarkFlagRequired("id")

	return cmd
}

func securityGroupDeleteRun(ctx context.Context, cmd *cobra.Command) error {
	if err := ado.Confirm(cmd, "Are you sure you want to delete this group?"); err != nil {
		return err
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

	// dev/team/commands.py:154: no table_transformer for this command.
	if err := client.Do(ctx, ado.Request{
		Method:     "DELETE",
		Host:       "vssps",
		Path:       "Graph/Groups/" + url.PathEscape(id),
		APIVersion: "5.0-preview.1",
	}, nil); err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	return ado.Print(cmd, nil)
}
