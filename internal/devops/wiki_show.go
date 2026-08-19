package devops

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

func wikiShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a wiki.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return wikiRunShow(context.Background(), cmd)
		},
	}

	cmd.Flags().String("wiki", "", "Name or Id of the wiki.")
	cmd.MarkFlagRequired("wiki")
	cmd.Flags().Bool("open", false, "Open the wiki in your web browser.")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func wikiRunShow(ctx context.Context, cmd *cobra.Command) error {
	wiki, _ := cmd.Flags().GetString("wiki")
	open, _ := cmd.Flags().GetBool("open")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return wikiShow(ctx, cmd, dctx, wiki, open)
}

// wikiShow does the actual client call, split out from wikiRunShow so tests
// can supply a dctx pointing at an httptest server without going through org
// validation.
func wikiShow(ctx context.Context, cmd *cobra.Command, dctx ado.Context, wiki string, open bool) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var w map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "wiki/wikis/" + url.PathEscape(wiki),
		APIVersion: "5.0",
	}, &w); err != nil {
		return fmt.Errorf("failed to get wiki: %w", err)
	}

	// wiki.py:111-112: webbrowser.open_new(url=wiki_object.remote_url) — no
	// extra HTTP call, launched after the API call and never suppresses the
	// printed output. A failure to launch is a warning, not a command failure
	// (foundation-spec.md §7.1).
	if open {
		if remoteURL := devopsStr(w["remoteUrl"]); remoteURL != "" {
			if err := ado.OpenBrowser(remoteURL); err != nil {
				logger.Warning("failed to open web browser: %v", err)
			}
		}
	}

	return ado.Print(cmd, w, wikiColumns...)
}
