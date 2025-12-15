package quota

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/quota/armquota"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

func Show(ctx context.Context, scope, resourceName, outputFormat string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	client, err := armquota.NewClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create quota client: %w", err)
	}

	quota, err := client.Get(ctx, resourceName, scope, nil)
	if err != nil {
		return fmt.Errorf("failed to get quota: %w", err)
	}

	if outputFormat == "json" {
		data, err := json.MarshalIndent(quota, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format quota: %w", err)
		}
		fmt.Println(string(data))
	} else {
		// Table output
		fmt.Printf("Name: %s\n", getStringValue(quota.Name))
		if quota.Properties != nil {
			props := quota.Properties
			if props.Limit != nil {
				if limitObj, ok := props.Limit.(*armquota.LimitObject); ok && limitObj.Value != nil {
					fmt.Printf("Limit: %d\n", *limitObj.Value)
				}
			}
			if props.Unit != nil {
				fmt.Printf("Unit: %s\n", *props.Unit)
			}
			if props.QuotaPeriod != nil {
				fmt.Printf("Quota Period: %s\n", *props.QuotaPeriod)
			}
			if props.IsQuotaApplicable != nil {
				fmt.Printf("Is Quota Applicable: %v\n", *props.IsQuotaApplicable)
			}
		}
	}

	return nil
}
