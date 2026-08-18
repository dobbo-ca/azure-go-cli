package encryptionset

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var resourceGroup string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List disk encryption sets",
		Long:  "List disk encryption sets in subscription or resource group",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			// list historically defaulted to table output, unlike the
			// global default of json, so only honor an explicitly-passed
			// format.
			format, _ := cmd.Flags().GetString("output")
			if !cmd.Flags().Changed("output") {
				format = "table"
			}
			return listDiskEncryptionSets(ctx, cmd, format, resourceGroup)
		},
	}

	cmd.Flags().StringVarP(&resourceGroup, "resource-group", "g", "", "Resource group name (lists all if not specified)")

	return cmd
}

func listDiskEncryptionSets(ctx context.Context, cmd *cobra.Command, format, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armcompute.NewDiskEncryptionSetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create disk encryption sets client: %w", err)
	}

	var sets []*armcompute.DiskEncryptionSet

	if resourceGroup != "" {
		// List by resource group
		pager := client.NewListByResourceGroupPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get page: %w", err)
			}
			sets = append(sets, page.Value...)
		}
	} else {
		// List all in subscription
		pager := client.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to get page: %w", err)
			}
			sets = append(sets, page.Value...)
		}
	}

	if format != "table" {
		return output.PrintJSON(cmd, sets)
	}

	// Table output
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tRESOURCE GROUP\tLOCATION\tPROVISIONING STATE")

	for _, set := range sets {
		name := ""
		if set.Name != nil {
			name = *set.Name
		}

		rg := getResourceGroupFromID(azure.GetStringValue(set.ID))

		location := ""
		if set.Location != nil {
			location = *set.Location
		}

		state := ""
		if set.Properties != nil && set.Properties.ProvisioningState != nil {
			state = *set.Properties.ProvisioningState
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, rg, location, state)
	}

	return w.Flush()
}

func getResourceGroupFromID(id string) string {
	if id == "" {
		return ""
	}

	// Parse resource ID to extract resource group
	// Format: /subscriptions/{sub}/resourceGroups/{rg}/...
	parts := []rune(id)
	start := -1
	slashCount := 0

	for i := 0; i < len(parts); i++ {
		if parts[i] == '/' {
			slashCount++
			if slashCount == 5 {
				start = i + 1
			} else if slashCount == 6 {
				return string(parts[start:i])
			}
		}
	}

	if start != -1 {
		return string(parts[start:])
	}

	return ""
}
