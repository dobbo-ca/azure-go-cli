package actiongroup

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

func Create(ctx context.Context, cmd *cobra.Command, resourceGroup, name, shortName string, emails, webhooks map[string]string, tags map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armmonitor.NewActionGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create action group client: %w", err)
	}

	var emailRecv []*armmonitor.EmailReceiver
	for k, v := range emails {
		emailRecv = append(emailRecv, &armmonitor.EmailReceiver{Name: to.Ptr(k), EmailAddress: to.Ptr(v)})
	}
	var webhookRecv []*armmonitor.WebhookReceiver
	for k, v := range webhooks {
		webhookRecv = append(webhookRecv, &armmonitor.WebhookReceiver{Name: to.Ptr(k), ServiceURI: to.Ptr(v)})
	}
	var azTags map[string]*string
	if len(tags) > 0 {
		azTags = make(map[string]*string, len(tags))
		for k, v := range tags {
			azTags[k] = to.Ptr(v)
		}
	}

	resp, err := client.CreateOrUpdate(ctx, resourceGroup, name, armmonitor.ActionGroupResource{
		Location: to.Ptr("Global"),
		Tags:     azTags,
		Properties: &armmonitor.ActionGroup{
			GroupShortName:   to.Ptr(shortName),
			Enabled:          to.Ptr(true),
			EmailReceivers:   emailRecv,
			WebhookReceivers: webhookRecv,
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create action group: %w", err)
	}
	return output.PrintJSON(cmd, resp.ActionGroupResource)
}
