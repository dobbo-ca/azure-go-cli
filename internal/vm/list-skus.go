package vm

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

type SKUInfo struct {
	Name         string   `json:"name"`
	ResourceType string   `json:"resourceType"`
	Tier         string   `json:"tier"`
	Size         string   `json:"size"`
	Family       string   `json:"family"`
	Locations    []string `json:"locations"`
	Restrictions string   `json:"restrictions"`
}

func ListSKUs(ctx context.Context, cmd *cobra.Command, location, sizeFilter, resourceType string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcompute.NewResourceSKUsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource SKUs client: %w", err)
	}

	// Normalize location to lowercase for comparison
	location = strings.ToLower(strings.ReplaceAll(location, " ", ""))

	skus := []SKUInfo{}
	pager := client.NewListPager(&armcompute.ResourceSKUsClientListOptions{
		Filter: nil,
	})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get SKUs page: %w", err)
		}

		for _, sku := range page.Value {
			// Skip if resource type doesn't match
			if sku.ResourceType != nil && *sku.ResourceType != resourceType {
				continue
			}

			// Check if SKU is available in the specified location
			locationMatch := false
			if sku.Locations != nil {
				for _, loc := range sku.Locations {
					if loc != nil && strings.ToLower(strings.ReplaceAll(*loc, " ", "")) == location {
						locationMatch = true
						break
					}
				}
			}

			if !locationMatch {
				continue
			}

			// Apply size filter if specified
			if sizeFilter != "" && sku.Name != nil && !strings.Contains(strings.ToLower(*sku.Name), strings.ToLower(sizeFilter)) {
				continue
			}

			// Check if SKU has location restrictions
			restrictionInfo := "None"
			if sku.Restrictions != nil && len(sku.Restrictions) > 0 {
				var restrictions []string
				for _, restriction := range sku.Restrictions {
					if restriction.Type != nil {
						restrictions = append(restrictions, string(*restriction.Type))
					}
				}
				if len(restrictions) > 0 {
					restrictionInfo = strings.Join(restrictions, ", ")
				}
			}

			locations := []string{}
			if sku.Locations != nil {
				for _, loc := range sku.Locations {
					if loc != nil {
						locations = append(locations, *loc)
					}
				}
			}

			skuInfo := SKUInfo{
				Name:         azure.GetStringValue(sku.Name),
				ResourceType: azure.GetStringValue(sku.ResourceType),
				Tier:         azure.GetStringValue(sku.Tier),
				Size:         azure.GetStringValue(sku.Size),
				Family:       azure.GetStringValue(sku.Family),
				Locations:    locations,
				Restrictions: restrictionInfo,
			}

			skus = append(skus, skuInfo)
		}
	}

	format, _ := cmd.Flags().GetString("output")
	// list-skus historically defaulted to table output, unlike the global
	// default of json, so only honor an explicitly-passed format.
	if !cmd.Flags().Changed("output") {
		format = "table"
	}

	if format != "table" {
		// PrintJSON (not PrintFormatted) keeps struct field declaration
		// order for "json" instead of alphabetizing keys, while still
		// delegating table/tsv/yaml/none to PrintFormatted itself. This
		// branch is only reached when the user explicitly passed a
		// non-table --output, so cmd's own flag value already matches
		// format.
		return output.PrintJSON(cmd, skus)
	}

	if len(skus) == 0 {
		fmt.Printf("No SKUs found for location '%s' with the specified filters\n", location)
		return nil
	}

	// Table output
	fmt.Printf("%-35s %-20s %-15s %-25s %-20s\n", "Name", "ResourceType", "Tier", "Locations", "Restrictions")
	fmt.Println(strings.Repeat("-", 120))
	for _, sku := range skus {
		locStr := strings.Join(sku.Locations, ", ")
		if len(locStr) > 25 {
			locStr = locStr[:22] + "..."
		}
		fmt.Printf("%-35s %-20s %-15s %-25s %-20s\n",
			sku.Name, sku.ResourceType, sku.Tier, locStr, sku.Restrictions)
	}
	fmt.Printf("\nTotal: %d SKUs\n", len(skus))

	return nil
}
