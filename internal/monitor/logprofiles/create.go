package logprofiles

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, name string, locations, categories []string, retentionDays int32, storageAccountID, serviceBusRuleID string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewLogProfilesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create log profiles client: %w", err)
	}

	props := &armmonitor.LogProfileProperties{
		Locations:  to.SliceOfPtrs(locations...),
		Categories: to.SliceOfPtrs(categories...),
		RetentionPolicy: &armmonitor.RetentionPolicy{
			Enabled: to.Ptr(retentionDays > 0),
			Days:    to.Ptr(retentionDays),
		},
	}
	if storageAccountID != "" {
		props.StorageAccountID = to.Ptr(storageAccountID)
	}
	if serviceBusRuleID != "" {
		props.ServiceBusRuleID = to.Ptr(serviceBusRuleID)
	}

	resp, err := client.CreateOrUpdate(ctx, name, armmonitor.LogProfileResource{
		Location:   to.Ptr("global"),
		Properties: props,
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create log profile: %w", err)
	}
	return output.PrintJSON(cmd, resp.LogProfileResource)
}
