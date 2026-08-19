package devops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// extensionMarketplaceURL is the fixed Marketplace endpoint search hits
// directly (extension.py:26), bypassing organization/auth entirely — a var,
// not a const, so tests can point it at an httptest server.
var extensionMarketplaceURL = "https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery"

func newExtensionSearchCmd() *cobra.Command {
	var searchQuery string

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search extensions in the marketplace.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionSearch(context.Background(), cmd, searchQuery)
		},
	}

	cmd.Flags().StringVarP(&searchQuery, "search-query", "q", "", "Search term")
	cmd.MarkFlagRequired("search-query")

	return cmd
}

// extensionSearchBody is the fixed VS Marketplace extensionquery request
// (extension.py:28-83): 9 hard-coded criteria filters (publisher/category
// constraints + filterType 12 value '37888' targeting Azure DevOps) plus one
// filterType-10 criterion carrying the user's search term.
func extensionSearchBody(searchQuery string) map[string]any {
	return map[string]any{
		"assetTypes": []string{
			"Microsoft.VisualStudio.Services.Icons.Default",
			"Microsoft.VisualStudio.Services.Icons.Branding",
			"Microsoft.VisualStudio.Services.Icons.Small",
		},
		"filters": []map[string]any{
			{
				"criteria": []map[string]any{
					{"filterType": 8, "value": "Microsoft.VisualStudio.Services"},
					{"filterType": 8, "value": "Microsoft.VisualStudio.Services.Integration"},
					{"filterType": 8, "value": "Microsoft.VisualStudio.Services.Cloud"},
					{"filterType": 8, "value": "Microsoft.TeamFoundation.Server"},
					{"filterType": 8, "value": "Microsoft.TeamFoundation.Server.Integration"},
					{"filterType": 8, "value": "Microsoft.VisualStudio.Services.Cloud.Integration"},
					{"filterType": 8, "value": "Microsoft.VisualStudio.Services.Resource.Cloud"},
					{"filterType": 10, "value": searchQuery},
					{"filterType": 12, "value": "37888"},
				},
				"direction":   2,
				"pageSize":    50,
				"pageNumber":  1,
				"sortBy":      0,
				"sortOrder":   0,
				"pagingToken": nil,
			},
		},
		"flags": 870,
	}
}

func runExtensionSearch(ctx context.Context, cmd *cobra.Command, searchQuery string) error {
	body, err := json.Marshal(extensionSearchBody(searchQuery))
	if err != nil {
		return fmt.Errorf("failed to build search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, extensionMarketplaceURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json;api-version=5.0-preview.1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to search extensions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to search extensions: marketplace returned status %d", resp.StatusCode)
	}

	var out struct {
		Results []struct {
			Extensions []map[string]any `json:"extensions"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("failed to decode search response: %w", err)
	}

	// Python indexes response_json['results'][0] unconditionally, which
	// crashes with an IndexError on an empty results list. Per port policy,
	// fixed rather than reproduced: an empty/absent results array just means
	// no matches.
	var extensions []map[string]any
	if len(out.Results) > 0 {
		extensions = out.Results[0].Extensions
	}

	return ado.Print(cmd, extensions, extensionSearchColumns...)
}
