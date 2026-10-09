package provider

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/spf13/cobra"
)

func newClient(cmd *cobra.Command) (*armresources.ProvidersClient, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}
	override, _ := cmd.Flags().GetString("subscription")
	subscriptionID, err := config.GetSubscription(override)
	if err != nil {
		return nil, err
	}
	client, err := armresources.NewProvidersClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create providers client: %w", err)
	}
	return client, nil
}

func NewProviderCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage resource providers",
	}

	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Register a provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, _ := cmd.Flags().GetString("namespace")
			mg, _ := cmd.Flags().GetString("management-group-id")
			wait, _ := cmd.Flags().GetBool("wait")
			consent, _ := cmd.Flags().GetBool("consent-to-permissions")
			return Register(context.Background(), cmd, namespace, mg, wait, consent)
		},
	}
	registerCmd.Flags().StringP("namespace", "n", "", "The resource namespace, aka 'provider'")
	registerCmd.Flags().StringP("management-group-id", "m", "", "The management group id to register")
	registerCmd.Flags().Bool("wait", false, "Wait for the registration to finish")
	registerCmd.Flags().BoolP("consent-to-permissions", "c", false, "A value indicating whether authorization is consented or not")
	registerCmd.MarkFlagRequired("namespace")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Gets a resource provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, _ := cmd.Flags().GetString("namespace")
			expand, _ := cmd.Flags().GetString("expand")
			return Show(context.Background(), cmd, namespace, expand)
		},
	}
	showCmd.Flags().StringP("namespace", "n", "", "The resource namespace, aka 'provider'")
	showCmd.Flags().String("expand", "", "The $expand query parameter, e.g. resourceTypes/aliases")
	showCmd.MarkFlagRequired("namespace")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Gets all resource providers for a subscription",
		RunE: func(cmd *cobra.Command, args []string) error {
			expand, _ := cmd.Flags().GetString("expand")
			return List(context.Background(), cmd, expand)
		},
	}
	listCmd.Flags().String("expand", "", "The properties to include in the results, e.g. metadata")

	cmd.AddCommand(registerCmd, showCmd, listCmd)
	return cmd
}

func expandOpt(expand string) *string {
	if expand == "" {
		return nil
	}
	return &expand
}
