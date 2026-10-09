package provider

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, namespace, expand string) error {
	client, err := newClient(cmd)
	if err != nil {
		return err
	}

	result, err := client.Get(ctx, namespace, &armresources.ProvidersClientGetOptions{Expand: expandOpt(expand)})
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	return output.PrintJSON(cmd, result.Provider)
}
