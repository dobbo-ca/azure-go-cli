package maintenanceconfiguration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List(ctx context.Context, clusterName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewMaintenanceConfigurationsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create maintenance configurations client: %w", err)
	}

	pager := client.NewListByManagedClusterPager(resourceGroup, clusterName, nil)
	var configs []map[string]interface{}

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list maintenance configurations: %w", err)
		}

		for _, config := range page.Value {
			configs = append(configs, formatConfig(config))
		}
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format maintenance configurations: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func formatConfig(config *armcontainerservice.MaintenanceConfiguration) map[string]interface{} {
	result := map[string]interface{}{
		"name": azure.GetStringValue(config.Name),
	}

	if config.Properties != nil {
		if config.Properties.TimeInWeek != nil && len(config.Properties.TimeInWeek) > 0 {
			windows := []map[string]interface{}{}
			for _, window := range config.Properties.TimeInWeek {
				w := map[string]interface{}{}
				if window.Day != nil {
					w["day"] = string(*window.Day)
				}
				if window.HourSlots != nil {
					w["hourSlots"] = window.HourSlots
				}
				windows = append(windows, w)
			}
			result["timeInWeek"] = windows
		}
		if config.Properties.NotAllowedTime != nil && len(config.Properties.NotAllowedTime) > 0 {
			notAllowed := []map[string]interface{}{}
			for _, na := range config.Properties.NotAllowedTime {
				n := map[string]interface{}{}
				if na.Start != nil {
					n["start"] = na.Start.String()
				}
				if na.End != nil {
					n["end"] = na.End.String()
				}
				notAllowed = append(notAllowed, n)
			}
			result["notAllowedTime"] = notAllowed
		}
	}

	return result
}
