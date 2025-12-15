package quota

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/quota/armquota"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

func RequestCreate(ctx context.Context, scope, resourceName string, limit int32, region string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	client, err := armquota.NewClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create quota client: %w", err)
	}

	// Create the quota request properties
	limitType := armquota.LimitTypeLimitValue

	properties := armquota.CurrentQuotaLimitBase{
		Properties: &armquota.Properties{
			Limit: &armquota.LimitObject{
				LimitObjectType: &limitType,
				Value:           &limit,
			},
			Name: &armquota.ResourceName{
				Value: &resourceName,
			},
		},
	}

	if region != "" {
		properties.Properties.ResourceType = &region
	}

	fmt.Printf("Submitting quota increase request for %s to limit %d...\n", resourceName, limit)

	// Begin the quota update operation (this is a long-running operation)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceName, scope, properties, nil)
	if err != nil {
		return fmt.Errorf("failed to begin quota update: %w", err)
	}

	fmt.Println("Quota request submitted. Waiting for response...")

	// Wait for the operation to complete
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete quota update: %w", err)
	}

	fmt.Println("\nQuota request completed successfully!")

	// Display the result
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format result: %w", err)
	}
	fmt.Println(string(data))

	return nil
}
