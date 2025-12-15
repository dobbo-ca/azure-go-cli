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
			return Login(context.Background(), tenantSelection)
		},
	}

	cmd.Flags().Bool("tenant-selection", false, "Always show tenant selection (useful with many subscriptions)")

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
