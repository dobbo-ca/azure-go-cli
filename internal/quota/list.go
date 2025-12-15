package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/quota/armquota"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

type QuotaInfo struct {
	Name         string `json:"name"`
	CurrentValue int32  `json:"currentValue"`
	Limit        int32  `json:"limit"`
	Unit         string `json:"unit"`
	QuotaPeriod  string `json:"quotaPeriod"`
}

func List(ctx context.Context, scope, outputFormat string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	client, err := armquota.NewClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create quota client: %w", err)
	}

	var quotas []QuotaInfo
	pager := client.NewListPager(scope, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get quota page: %w", err)
		}

		for _, quota := range page.Value {
			if quota.Properties == nil {
				continue
			}

			props := quota.Properties
			quotaInfo := QuotaInfo{
				Name: getStringValue(quota.Name),
				Unit: getStringValue(props.Unit),
			}

			// Extract limit value from LimitObject
			if props.Limit != nil {
				if limitObj, ok := props.Limit.(*armquota.LimitObject); ok && limitObj.Value != nil {
					quotaInfo.Limit = *limitObj.Value
				}
			}

			if props.QuotaPeriod != nil {
				quotaInfo.QuotaPeriod = *props.QuotaPeriod
			}

			quotas = append(quotas, quotaInfo)
		}
	}

	if len(quotas) == 0 {
		fmt.Printf("No quotas found for scope '%s'\n", scope)
		return nil
	}

	if outputFormat == "json" {
		data, err := json.MarshalIndent(quotas, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format quotas: %w", err)
		}
		fmt.Println(string(data))
	} else {
		// Table output
		fmt.Printf("%-40s %-15s %-15s %-15s %-20s\n", "Name", "Current", "Limit", "Unit", "QuotaPeriod")
		fmt.Println(strings.Repeat("-", 110))
		for _, quota := range quotas {
			fmt.Printf("%-40s %-15d %-15d %-15s %-20s\n",
				quota.Name, quota.CurrentValue, quota.Limit, quota.Unit, quota.QuotaPeriod)
		}
		fmt.Printf("\nTotal: %d quotas\n", len(quotas))
	}

	return nil
}

func getStringValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
