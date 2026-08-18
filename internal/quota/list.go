package quota

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/quota/armquota"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

type QuotaInfo struct {
	Name         string `json:"name"`
	CurrentValue int32  `json:"currentValue"`
	Limit        int32  `json:"limit"`
	Unit         string `json:"unit"`
	QuotaPeriod  string `json:"quotaPeriod"`
}

func List(ctx context.Context, cmd *cobra.Command, scope, outputFormat string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	client, err := armquota.NewClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create quota client: %w", err)
	}

	quotas := []QuotaInfo{}
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
				Name: azure.GetStringValue(quota.Name),
				Unit: azure.GetStringValue(props.Unit),
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

	if outputFormat == "table" {
		if len(quotas) == 0 {
			fmt.Printf("No quotas found for scope '%s'\n", scope)
			return nil
		}
		fmt.Printf("%-40s %-15s %-15s %-15s %-20s\n", "Name", "Current", "Limit", "Unit", "QuotaPeriod")
		fmt.Println(strings.Repeat("-", 110))
		for _, quota := range quotas {
			fmt.Printf("%-40s %-15d %-15d %-15s %-20s\n",
				quota.Name, quota.CurrentValue, quota.Limit, quota.Unit, quota.QuotaPeriod)
		}
		fmt.Printf("\nTotal: %d quotas\n", len(quotas))
	} else {
		// PrintJSON keeps struct field declaration order for "json" instead
		// of alphabetizing keys, while still delegating table/tsv/yaml/none
		// to PrintFormatted itself.
		return output.PrintJSON(cmd, quotas)
	}

	return nil
}
