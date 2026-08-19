package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

func wikiPageShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get the content of a page or open a page.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return wikiRunPageShow(context.Background(), cmd)
		},
	}

	cmd.Flags().String("wiki", "", "Name or Id of the wiki.")
	cmd.MarkFlagRequired("wiki")
	cmd.Flags().String("path", "", "Path of the wiki page.")
	cmd.MarkFlagRequired("path")
	// --version/-v: see the same argument-context note in wiki_page_update.go.
	cmd.Flags().StringP("version", "v", "", "Version (ETag) of the wiki page.")
	cmd.Flags().Bool("open", false, "Open the wiki page in your web browser.")
	cmd.Flags().Bool("include-content", false, "Include content of the page.")
	cmd.Flags().String("recursion-level", "", "Include subpages of the page.")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func wikiRunPageShow(ctx context.Context, cmd *cobra.Command) error {
	wiki, _ := cmd.Flags().GetString("wiki")
	path, _ := cmd.Flags().GetString("path")
	version, _ := cmd.Flags().GetString("version")
	open, _ := cmd.Flags().GetBool("open")
	includeContent, _ := cmd.Flags().GetBool("include-content")
	recursionLevel, _ := cmd.Flags().GetString("recursion-level")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return wikiPageShow(ctx, cmd, dctx, wiki, path, version, open, includeContent, recursionLevel)
}

// wikiPageShow does the actual client call, split out from wikiRunPageShow
// so tests can supply a dctx pointing at an httptest server without going
// through org validation.
func wikiPageShow(ctx context.Context, cmd *cobra.Command, dctx ado.Context, wiki, path, version string, open, includeContent bool, recursionLevel string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("path", path)
	if recursionLevel != "" {
		q.Set("recursionLevel", recursionLevel)
	}
	if includeContent {
		q.Set("includeContent", "true")
	} else {
		q.Set("includeContent", "false")
	}
	// wiki.py:204-207 passes --version straight through as get_page's
	// version_descriptor=<str>, but the SDK expects a GitVersionDescriptor
	// object and reads .version_type off it (wiki_client.py:174-180) — on a
	// plain string that's an AttributeError, i.e. Python crashes here. Per
	// this port's bug policy (fix crashes, don't reproduce them) this sends
	// what was clearly intended: an ETag-scoped versionDescriptor.version
	// query param. versionType/versionOptions are left unset, same as
	// Python's own GitVersionDescriptor(version=<str>) would have produced.
	if version != "" {
		q.Set("versionDescriptor.version", version)
	}

	page, etag, err := wikiDoPage(ctx, client, ado.Request{
		Scope:      dctx.Project,
		Path:       "wiki/wikis/" + url.PathEscape(wiki) + "/pages",
		APIVersion: "5.0",
		Query:      q,
	}, "")
	if err != nil {
		return fmt.Errorf("failed to get wiki page: %w", err)
	}

	if open {
		if remoteURL := devopsStr(page["remoteUrl"]); remoteURL != "" {
			if err := ado.OpenBrowser(remoteURL); err != nil {
				logger.Warning("failed to open web browser: %v", err)
			}
		}
	}

	return ado.Print(cmd, map[string]any{"eTag": etag, "page": page}, wikiPageColumns...)
}
