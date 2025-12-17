package flexibleserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func ListSKUs(ctx context.Context, location string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewLocationBasedCapabilitiesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create location based capabilities client: %w", err)
	}

	pager := client.NewExecutePager(location, nil)
	var skus []map[string]interface{}

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list SKUs: %w", err)
		}

		for _, capability := range page.Value {
			if capability.SupportedServerEditions != nil {
				for _, edition := range capability.SupportedServerEditions {
					if edition.SupportedServerSKUs != nil {
						for _, sku := range edition.SupportedServerSKUs {
							skus = append(skus, formatSKU(sku, edition, capability.SupportedServerVersions))
						}
					}
				}
			}
		}
	}

	data, err := json.MarshalIndent(skus, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format SKUs: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func formatSKU(sku *armpostgresqlflexibleservers.ServerSKUCapability, edition *armpostgresqlflexibleservers.FlexibleServerEditionCapability, versions []*armpostgresqlflexibleservers.ServerVersionCapability) map[string]interface{} {
	result := map[string]interface{}{}

	if sku.Name != nil {
		result["name"] = *sku.Name
	}

	if edition.Name != nil {
		result["tier"] = *edition.Name
	}

	// List all supported versions
	if versions != nil {
		versionNames := []string{}
		for _, v := range versions {
			if v.Name != nil {
				versionNames = append(versionNames, *v.Name)
			}
		}
		if len(versionNames) > 0 {
			result["supportedVersions"] = versionNames
		}
	}

	if sku.VCores != nil {
		result["vCores"] = *sku.VCores
	}

	if sku.SupportedMemoryPerVcoreMb != nil {
		result["memoryPerVcoreMB"] = *sku.SupportedMemoryPerVcoreMb
	}

	if sku.SupportedIops != nil {
		result["supportedIops"] = *sku.SupportedIops
	}

	if sku.Status != nil {
		result["status"] = *sku.Status
	}

	return result
}
