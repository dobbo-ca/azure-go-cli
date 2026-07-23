package auth

import (
	"context"

	"github.com/spf13/cobra"
)

func NewLoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Azure",
		Long:  "Log in to Azure using device code flow",
		RunE: func(cmd *cobra.Command, args []string) error {
			tenantSelection, _ := cmd.Flags().GetBool("tenant-selection")
			subscription, _ := cmd.Flags().GetString("subscription")
			tenant, _ := cmd.Flags().GetString("tenant")
			return Login(context.Background(), tenantSelection, subscription, tenant)
		},
	}

	cmd.Flags().Bool("tenant-selection", false, "Always show tenant selection (useful with many subscriptions)")
	cmd.Flags().StringP("tenant", "t", "", "Tenant ID or domain to sign in to (prompts for subscription unless --subscription is given)")

	return cmd
}

func NewLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Aliases: []string{"logoff"},
		Short:   "Log out from Azure",
		Long:    "Clear stored Azure credentials and log out",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Logout()
		},
	}
}
