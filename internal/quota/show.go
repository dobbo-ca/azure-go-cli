package quota

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/quota/armquota"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, scope, resourceName, outputFormat string) error {
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

	if outputFormat == "table" {
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
	} else {
		// PrintJSON keeps struct field declaration order for "json" instead
		// of alphabetizing keys, while still delegating table/tsv/yaml/none
		// to PrintFormatted itself. (armquota.CurrentUsagesBase's generated
		// MarshalJSON already builds a map, so this is a no-op fix here, but
		// keeping it consistent with the sibling quota commands.)
		return output.PrintJSON(cmd, quota)
	}

	return nil
}
