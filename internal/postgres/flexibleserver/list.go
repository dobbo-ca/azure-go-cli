package flexibleserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List(ctx context.Context, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewServersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create postgresql flexible servers client: %w", err)
	}

	var servers []map[string]interface{}

	if resourceGroup != "" {
		// List servers in specific resource group
		pager := client.NewListByResourceGroupPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list postgresql flexible servers: %w", err)
			}

			for _, server := range page.Value {
				servers = append(servers, formatServer(server))
			}
		}
	} else {
		// List all servers in subscription
		pager := client.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list postgresql flexible servers: %w", err)
			}

			for _, server := range page.Value {
				servers = append(servers, formatServer(server))
			}
		}
	}

	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format postgresql flexible servers: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func formatServer(server *armpostgresqlflexibleservers.Server) map[string]interface{} {
	result := map[string]interface{}{
		"name":          azure.GetStringValue(server.Name),
		"location":      azure.GetStringValue(server.Location),
		"resourceGroup": getResourceGroupFromID(azure.GetStringValue(server.ID)),
	}

	if server.SKU != nil {
		if server.SKU.Name != nil {
			result["sku"] = *server.SKU.Name
		}
		if server.SKU.Tier != nil {
			result["tier"] = string(*server.SKU.Tier)
		}
	}

	if server.Properties != nil {
		if server.Properties.Version != nil {
			result["version"] = string(*server.Properties.Version)
		}
		if server.Properties.State != nil {
			result["state"] = string(*server.Properties.State)
		}
		if server.Properties.FullyQualifiedDomainName != nil {
			result["fullyQualifiedDomainName"] = *server.Properties.FullyQualifiedDomainName
		}
	}

	return result
}

func getResourceGroupFromID(id string) string {
	// ID format: /subscriptions/{sub}/resourceGroups/{rg}/providers/...
	parts := make([]string, 0)
	for _, part := range []rune(id) {
		if part == '/' {
			parts = append(parts, "")
		} else if len(parts) > 0 {
			parts[len(parts)-1] += string(part)
		}
	}

	for i, part := range parts {
		if part == "resourceGroups" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
