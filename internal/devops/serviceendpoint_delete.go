package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func serviceendpointNewDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes service endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serviceendpointRunDelete(context.Background(), cmd)
		},
	}

	cmd.Flags().String("id", "", "Id of the service endpoint to delete.")
	_ = cmd.MarkFlagRequired("id")
	cmd.Flags().Bool("deep", false, "Specific to AzureRM endpoint created in Automatic flow. "+
		"When it is specified, this will also delete corresponding AAD application in Azure.")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddYesFlag(cmd)

	return cmd
}

func serviceendpointRunDelete(ctx context.Context, cmd *cobra.Command) error {
	if err := ado.Confirm(cmd, "Are you sure you want to delete this service-endpoint?"); err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")
	deep, _ := cmd.Flags().GetBool("deep")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("deep", strconv.FormatBool(deep))

	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodDelete,
		Scope:      dctx.Project,
		Path:       "serviceendpoint/endpoints/" + url.PathEscape(id),
		APIVersion: "5.0-preview.2",
		Query:      q,
	}, nil); err != nil {
		return fmt.Errorf("failed to delete service endpoint: %w", err)
	}

	// service_endpoint_client.py:84-88: no return value, no table transformer.
	return nil
}
