package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func wikiDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a wiki.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return wikiRunDelete(context.Background(), cmd)
		},
	}

	cmd.Flags().String("wiki", "", "Name or Id of the wiki to delete.")
	cmd.MarkFlagRequired("wiki")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)

	return cmd
}

func wikiRunDelete(ctx context.Context, cmd *cobra.Command) error {
	wiki, _ := cmd.Flags().GetString("wiki")

	if err := ado.Confirm(cmd, "Are you sure you want to delete this wiki?"); err != nil {
		return err
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return wikiDelete(ctx, cmd, dctx, wiki)
}

// wikiDelete does the actual client call, split out from wikiRunDelete so
// tests can supply a dctx pointing at an httptest server without going
// through org validation.
func wikiDelete(ctx context.Context, cmd *cobra.Command, dctx ado.Context, wiki string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var w map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Scope:      dctx.Project,
		Path:       "wiki/wikis/" + url.PathEscape(wiki),
		APIVersion: "5.0",
	}, &w); err != nil {
		return fmt.Errorf("failed to delete wiki: %w", err)
	}

	// commands.py:177: devops wiki delete does register transform_wiki_table_output,
	// unlike some other delete commands in this extension.
	return ado.Print(cmd, w, wikiColumns...)
}
