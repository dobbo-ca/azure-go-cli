package image

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// parseURN splits publisher:offer:sku:version.
func parseURN(urn string) (publisher, offer, sku, version string, err error) {
	items := strings.Split(urn, ":")
	if len(items) != 4 {
		return "", "", "", "", fmt.Errorf("--urn should be in the format of publisher:offer:sku:version")
	}
	return items[0], items[1], items[2], items[3], nil
}

func subscriptionID(cmd *cobra.Command) (string, error) {
	sub, _ := cmd.Flags().GetString("subscription")
	id, err := config.GetSubscription(sub)
	if err != nil {
		return "", fmt.Errorf("failed to get subscription: %w", err)
	}
	return id, nil
}

// firstLocation mirrors Python's get_one_of_subscription_locations (prefers westus).
func firstLocation(ctx context.Context, cred azcore.TokenCredential, subID string) (string, error) {
	client, err := armsubscriptions.NewClient(cred, nil)
	if err != nil {
		return "", err
	}
	pager := client.NewListLocationsPager(subID, nil)
	first := ""
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list locations: %w", err)
		}
		for _, l := range page.Value {
			n := azure.GetStringValue(l.Name)
			if strings.EqualFold(n, "westus") {
				return n, nil
			}
			if first == "" {
				first = n
			}
		}
	}
	if first == "" {
		return "", fmt.Errorf("Current subscription does not have valid location list")
	}
	return first, nil
}

// getImage resolves the image from --urn or the individual parts.
func getImage(ctx context.Context, cmd *cobra.Command, urn, location, publisher, offer, sku, version string) (*armcompute.VirtualMachineImage, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}
	subID, err := subscriptionID(cmd)
	if err != nil {
		return nil, err
	}
	if location == "" {
		if location, err = firstLocation(ctx, cred, subID); err != nil {
			return nil, err
		}
	}
	if urn != "" {
		if publisher != "" || offer != "" || sku != "" || version != "" {
			return nil, fmt.Errorf("--urn is mutually exclusive with --publisher, --offer, --sku and --version")
		}
		if publisher, offer, sku, version, err = parseURN(urn); err != nil {
			return nil, err
		}
	} else if publisher == "" || offer == "" || sku == "" || version == "" {
		return nil, fmt.Errorf("please specify all of (--publisher, --offer, --sku, --version), or --urn")
	}

	client, err := armcompute.NewVirtualMachineImagesClient(subID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create image client: %w", err)
	}
	if strings.EqualFold(version, "latest") {
		top := int32(1)
		orderby := "name desc"
		list, err := client.List(ctx, location, publisher, offer, sku, &armcompute.VirtualMachineImagesClientListOptions{Top: &top, Orderby: &orderby})
		if err != nil {
			return nil, fmt.Errorf("failed to list image versions: %w", err)
		}
		if len(list.VirtualMachineImageResourceArray) == 0 {
			return nil, fmt.Errorf("can't resolve the version of '%s:%s:%s'", publisher, offer, sku)
		}
		version = azure.GetStringValue(list.VirtualMachineImageResourceArray[0].Name)
	}
	res, err := client.Get(ctx, location, publisher, offer, sku, version, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}
	return &res.VirtualMachineImage, nil
}

func Show(ctx context.Context, cmd *cobra.Command, urn, location, publisher, offer, sku, version string) error {
	img, err := getImage(ctx, cmd, urn, location, publisher, offer, sku, version)
	if err != nil {
		return err
	}
	// Python hoists properties (incl. plan) to the top level.
	raw, err := json.Marshal(img)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	return output.PrintJSON(cmd, flatten(m))
}
