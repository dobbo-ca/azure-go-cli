package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// wikiPageDefaultUpdateComment is _DEFAULT_PAGE_UPDATE_MESSAGE (dev/team/wiki.py:17).
const wikiPageDefaultUpdateComment = "Updated the page using Azure DevOps CLI"

func wikiPageUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Edit a page.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return wikiRunPageUpdate(context.Background(), cmd)
		},
	}

	wikiAddPageFlags(cmd, wikiPageDefaultUpdateComment)
	// --version/-v: "devops wiki" argument_context registers this alias
	// (arguments.py:185) and applies here too since "devops wiki" is a
	// string-prefix of "devops wiki page update" in knack's argument-context
	// matching (foundation-spec.md §1, admin-banner precedent).
	cmd.Flags().StringP("version", "v", "", "Version (ETag) of file to edit.")
	cmd.MarkFlagRequired("version")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func wikiRunPageUpdate(ctx context.Context, cmd *cobra.Command) error {
	wiki, _ := cmd.Flags().GetString("wiki")
	path, _ := cmd.Flags().GetString("path")
	comment, _ := cmd.Flags().GetString("comment")
	version, _ := cmd.Flags().GetString("version")

	content, err := wikiPageContent(cmd)
	if err != nil {
		return err
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return wikiPageUpdate(ctx, cmd, dctx, wiki, path, comment, content, version)
}

// wikiPageUpdate does the actual client call, split out from
// wikiRunPageUpdate so tests can supply a dctx pointing at an httptest
// server without going through org validation.
func wikiPageUpdate(ctx context.Context, cmd *cobra.Command, dctx ado.Context, wiki, path, comment, content, version string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("path", path)
	q.Set("comment", comment)

	// Same route/verb as create (commands.py:182 registers this as "update",
	// not a different HTTP method) — only If-Match differs.
	page, etag, err := wikiDoPage(ctx, client, ado.Request{
		Method:     http.MethodPut,
		Scope:      dctx.Project,
		Path:       "wiki/wikis/" + url.PathEscape(wiki) + "/pages",
		APIVersion: "5.0",
		Query:      q,
		Body:       map[string]any{"content": content},
	}, version)
	if err != nil {
		return fmt.Errorf("failed to update wiki page: %w", err)
	}

	return ado.Print(cmd, map[string]any{"eTag": etag, "page": page}, wikiPageColumns...)
}
