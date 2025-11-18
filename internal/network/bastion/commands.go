package bastion

import (
  "context"

  "github.com/spf13/cobra"
)

func NewBastionCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "bastion",
    Short: "Manage Azure Bastion resources",
    Long:  "Commands to manage Azure Bastion",
  }

  tunnelCmd := &cobra.Command{
    Use:   "tunnel",
    Short: "Open tunnel to a target resource through Azure Bastion",
    RunE: func(cmd *cobra.Command, args []string) error {
      bastionName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      targetResourceID, _ := cmd.Flags().GetString("target-resource-id")
      resourcePort, _ := cmd.Flags().GetInt("resource-port")
      localPort, _ := cmd.Flags().GetInt("port")
      bufferSize, _ := cmd.Flags().GetInt("buffer-size")

      return Tunnel(context.Background(), bastionName, resourceGroup, targetResourceID, resourcePort, localPort, bufferSize)
    },
  }
  tunnelCmd.Flags().StringP("name", "n", "", "Bastion name")
  tunnelCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  tunnelCmd.Flags().String("target-resource-id", "", "Target resource ID")
  tunnelCmd.Flags().Int("resource-port", 443, "Target resource port")
  tunnelCmd.Flags().IntP("port", "p", 0, "Local port")
  tunnelCmd.Flags().Int("buffer-size", 32*1024, "WebSocket buffer size in bytes (default 32KB)")
  tunnelCmd.MarkFlagRequired("name")
  tunnelCmd.MarkFlagRequired("resource-group")
  tunnelCmd.MarkFlagRequired("target-resource-id")
  tunnelCmd.MarkFlagRequired("port")

  sshCmd := &cobra.Command{
    Use:   "ssh",
    Short: "Open SSH session to a VM through Azure Bastion",
    Long: `Open SSH session to a VM through Azure Bastion.

This command creates a tunnel through Azure Bastion and launches an SSH session.
Requires ssh client to be installed.

For AAD authentication, provide your Azure AD username (typically your email or UPN).`,
    RunE: func(cmd *cobra.Command, args []string) error {
      bastionName, _ := cmd.Flags().GetString("name")
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      targetResourceID, _ := cmd.Flags().GetString("target-resource-id")
      authType, _ := cmd.Flags().GetString("auth-type")
      username, _ := cmd.Flags().GetString("username")
      bufferSize, _ := cmd.Flags().GetInt("buffer-size")

      return SSH(context.Background(), bastionName, resourceGroup, targetResourceID, authType, username, bufferSize)
    },
  }
  sshCmd.Flags().StringP("name", "n", "", "Bastion name")
  sshCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  sshCmd.Flags().String("target-resource-id", "", "Target VM resource ID")
  sshCmd.Flags().String("auth-type", "AAD", "Authentication type (AAD, password, ssh-key)")
  sshCmd.Flags().StringP("username", "u", "", "SSH username (Azure AD email for AAD auth)")
  sshCmd.Flags().Int("buffer-size", 32*1024, "WebSocket buffer size in bytes (default 32KB)")
  sshCmd.MarkFlagRequired("name")
  sshCmd.MarkFlagRequired("resource-group")
  sshCmd.MarkFlagRequired("target-resource-id")

  cmd.AddCommand(tunnelCmd, sshCmd)
  return cmd
}
