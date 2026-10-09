package sig

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command) error {
	f := cmd.Flags()
	rg, _ := f.GetString("resource-group")
	name, _ := f.GetString("gallery-name")
	sel, _ := f.GetString("select")
	groups, _ := f.GetBool("sharing-groups")

	opts := &armcompute.GalleriesClientGetOptions{}
	if sel != "" {
		s, err := enumValue("select", sel, "Permissions")
		if err != nil {
			return err
		}
		opts.Select = to.Ptr(armcompute.SelectPermissions(s))
	}
	if groups {
		opts.Expand = to.Ptr(armcompute.GalleryExpandParamsSharingProfileGroups)
	}

	client, _, _, err := newClients(cmd)
	if err != nil {
		return err
	}
	resp, err := client.Get(ctx, rg, name, opts)
	if err != nil {
		return fmt.Errorf("failed to get gallery: %w", err)
	}
	return output.PrintJSON(cmd, resp.Gallery)
}
