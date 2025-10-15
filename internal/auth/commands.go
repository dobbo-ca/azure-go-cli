package auth

import (
  "context"

  "github.com/spf13/cobra"
)

func NewLoginCommand() *cobra.Command {
  return &cobra.Command{
    Use:   "login",
    Short: "Log in to Azure",
    Long:  "Log in to Azure using device code flow",
    RunE: func(cmd *cobra.Command, args []string) error {
      return Login(context.Background())
    },
  }
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
