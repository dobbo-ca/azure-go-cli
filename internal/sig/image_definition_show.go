package sig

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func ShowImageDefinition(ctx context.Context, cmd *cobra.Command) error {
	f := cmd.Flags()
	rg, _ := f.GetString("resource-group")
	gallery, _ := f.GetString("gallery-name")
	name, _ := f.GetString("gallery-image-definition")

	_, client, _, err := newClients(cmd)
	if err != nil {
		return err
	}
	resp, err := client.Get(ctx, rg, gallery, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get image definition: %w", err)
	}
	return output.PrintJSON(cmd, resp.GalleryImage)
}
