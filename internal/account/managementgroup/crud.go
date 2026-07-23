package managementgroup

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/managementgroups/armmanagementgroups"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// List returns the management groups visible in the current tenant.
func List(ctx context.Context, cmd *cobra.Command) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	groups := []MGInfo{}
	pager := client.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list management groups: %w", err)
		}
		for _, g := range page.Value {
			groups = append(groups, listItemInfo(g))
		}
	}

	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, groups, format)
}

// Show returns a single management group by name (group ID).
func Show(ctx context.Context, cmd *cobra.Command, name string) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	resp, err := client.Get(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get management group %q: %w", name, err)
	}

	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, groupInfo(&resp.ManagementGroup), format)
}

// Create creates a management group. This is a long-running operation.
func Create(ctx context.Context, name, displayName, parent string) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	props := &armmanagementgroups.CreateManagementGroupProperties{}
	if displayName != "" {
		props.DisplayName = to.Ptr(displayName)
	}
	if pid := normalizeParentID(parent); pid != "" {
		props.Details = &armmanagementgroups.CreateManagementGroupDetails{
			Parent: &armmanagementgroups.CreateParentGroupInfo{ID: to.Ptr(pid)},
		}
	}

	poller, err := client.BeginCreateOrUpdate(ctx, name, armmanagementgroups.CreateManagementGroupRequest{
		Name:       to.Ptr(name),
		Properties: props,
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create management group %q: %w", name, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to create management group %q: %w", name, err)
	}

	fmt.Printf("Created management group '%s'\n", name)
	return nil
}

// Delete removes a management group. This is a long-running operation.
func Delete(ctx context.Context, name string) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	poller, err := client.BeginDelete(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to delete management group %q: %w", name, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to delete management group %q: %w", name, err)
	}

	fmt.Printf("Deleted management group '%s'\n", name)
	return nil
}

// Update changes a management group's display name and/or parent.
func Update(ctx context.Context, cmd *cobra.Command, name, displayName, parent string) error {
	client, err := newClient()
	if err != nil {
		return err
	}

	patch := armmanagementgroups.PatchManagementGroupRequest{}
	if displayName != "" {
		patch.DisplayName = to.Ptr(displayName)
	}
	if pid := normalizeParentID(parent); pid != "" {
		patch.ParentGroupID = to.Ptr(pid)
	}

	resp, err := client.Update(ctx, name, patch, nil)
	if err != nil {
		return fmt.Errorf("failed to update management group %q: %w", name, err)
	}

	format, _ := cmd.Flags().GetString("output")
	return output.PrintFormatted(cmd, groupInfo(&resp.ManagementGroup), format)
}
