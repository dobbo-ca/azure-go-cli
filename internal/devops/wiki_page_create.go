package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// wikiPageDefaultAddComment is _DEFAULT_PAGE_ADD_MESSAGE (dev/team/wiki.py:16).
const wikiPageDefaultAddComment = "Added a new page using Azure DevOps CLI"

func wikiPageCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Add a new page.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return wikiRunPageCreate(context.Background(), cmd)
		},
	}

	wikiAddPageFlags(cmd, wikiPageDefaultAddComment)

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

// wikiAddPageFlags registers the flags shared by "wiki page create" and
// "wiki page update": --wiki, --path, --comment (with a per-command default),
// --content, --file-path, --encoding.
func wikiAddPageFlags(cmd *cobra.Command, defaultComment string) {
	cmd.Flags().String("wiki", "", "Name or Id of the wiki.")
	cmd.MarkFlagRequired("wiki")
	cmd.Flags().String("path", "", "Path of the wiki page.")
	cmd.MarkFlagRequired("path")
	cmd.Flags().String("comment", defaultComment, "Comment to be associated with this page operation.")
	cmd.Flags().String("content", "", "Content of the wiki page. Ignored if --file-path is specified.")
	cmd.Flags().String("file-path", "", "Path of the file input if content is specified in the file.")
	cmd.Flags().String("encoding", "utf-8", "Encoding of the file. Used in conjunction with --file-path parameter. Allowed values: ascii, utf-16be, utf-16le, utf-8.")
}

// wikiPageContent resolves --content/--file-path per wiki.py:132-144: content
// wins if both are given, either is required.
func wikiPageContent(cmd *cobra.Command) (string, error) {
	content, _ := cmd.Flags().GetString("content")
	if content != "" {
		return content, nil
	}
	filePath, _ := cmd.Flags().GetString("file-path")
	if filePath == "" {
		return "", fmt.Errorf("Either --file-path or --content must be specified.")
	}
	encoding, _ := cmd.Flags().GetString("encoding")
	return wikiReadFileContent(filePath, encoding)
}

func wikiRunPageCreate(ctx context.Context, cmd *cobra.Command) error {
	wiki, _ := cmd.Flags().GetString("wiki")
	path, _ := cmd.Flags().GetString("path")
	comment, _ := cmd.Flags().GetString("comment")

	content, err := wikiPageContent(cmd)
	if err != nil {
		return err
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return wikiPageCreate(ctx, cmd, dctx, wiki, path, comment, content)
}

// wikiPageCreate does the actual client call, split out from
// wikiRunPageCreate so tests can supply a dctx pointing at an httptest
// server without going through org validation.
func wikiPageCreate(ctx context.Context, cmd *cobra.Command, dctx ado.Context, wiki, path, comment, content string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("path", path)
	q.Set("comment", comment)

	// create always passes version=None to the SDK, so no If-Match header is
	// sent (wiki_client.py:111-113) — "" here means none.
	page, etag, err := wikiDoPage(ctx, client, ado.Request{
		Method:     http.MethodPut,
		Scope:      dctx.Project,
		Path:       "wiki/wikis/" + url.PathEscape(wiki) + "/pages",
		APIVersion: "5.0",
		Query:      q,
		Body:       map[string]any{"content": content},
	}, "")
	if err != nil {
		return fmt.Errorf("failed to create wiki page: %w", err)
	}

	return ado.Print(cmd, map[string]any{"eTag": etag, "page": page}, wikiPageColumns...)
}
