package serverlogs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Download(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName string, names []string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armpostgresqlflexibleservers.NewLogFilesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create log files client: %w", err)
	}

	var items []*armpostgresqlflexibleservers.LogFile
	pager := client.NewListByServerPager(resourceGroup, serverName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list log files: %w", err)
		}
		items = append(items, page.Value...)
	}

	filter := make(map[string]bool, len(names))
	for _, n := range names {
		filter[n] = true
	}

	downloaded := []string{}
	for _, item := range items {
		if item == nil {
			continue
		}
		name := azure.GetStringValue(item.Name)
		if len(filter) > 0 && !filter[name] {
			continue
		}
		if item.Properties == nil || item.Properties.URL == nil {
			continue
		}
		url := azure.GetStringValue(item.Properties.URL)
		if err := downloadFile(ctx, url, name); err != nil {
			return fmt.Errorf("failed to download log file '%s': %w", name, err)
		}
		fmt.Printf("Downloaded '%s'\n", name)
		downloaded = append(downloaded, name)
	}

	return output.PrintJSON(cmd, map[string]interface{}{
		"status":     "download complete",
		"downloaded": downloaded,
	})
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}
