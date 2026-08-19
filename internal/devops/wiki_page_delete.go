package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// wikiPageDefaultDeleteComment is _DEFAULT_PAGE_DELETE_MESSAGE (dev/team/wiki.py:18).
const wikiPageDefaultDeleteComment = "Deleted the page using Azure DevOps CLI"

func wikiPageDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a page.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return wikiRunPageDelete(context.Background(), cmd)
		},
	}

	cmd.Flags().String("wiki", "", "Name or Id of the wiki.")
	cmd.MarkFlagRequired("wiki")
	cmd.Flags().String("path", "", "Path of the wiki page.")
	cmd.MarkFlagRequired("path")
	cmd.Flags().String("comment", wikiPageDefaultDeleteComment, "Comment to be associated with this page delete.")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)
	ado.AddYesFlag(cmd)

	return cmd
}

func wikiRunPageDelete(ctx context.Context, cmd *cobra.Command) error {
	wiki, _ := cmd.Flags().GetString("wiki")
	path, _ := cmd.Flags().GetString("path")
	comment, _ := cmd.Flags().GetString("comment")

	if err := ado.Confirm(cmd, "Are you sure you want to delete this wiki page?"); err != nil {
		return err
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return wikiPageDelete(ctx, cmd, dctx, wiki, path, comment)
}

// wikiPageDelete does the actual client call, split out from
// wikiRunPageDelete so tests can supply a dctx pointing at an httptest
// server without going through org validation.
func wikiPageDelete(ctx context.Context, cmd *cobra.Command, dctx ado.Context, wiki, path, comment string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("path", path)
	q.Set("comment", comment)

	page, etag, err := wikiDoPage(ctx, client, ado.Request{
		Method:     http.MethodDelete,
		Scope:      dctx.Project,
		Path:       "wiki/wikis/" + url.PathEscape(wiki) + "/pages",
		APIVersion: "5.0",
		Query:      q,
	}, "")
	if err != nil {
		return fmt.Errorf("failed to delete wiki page: %w", err)
	}

	// commands.py:184: no table_transformer for delete, unlike create/update/show
	// — table mode falls back to JSON since we pass no columns.
	return ado.Print(cmd, map[string]any{"eTag": etag, "page": page})
}
