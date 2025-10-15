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

      return Tunnel(context.Background(), bastionName, resourceGroup, targetResourceID, resourcePort, localPort)
    },
  }
  tunnelCmd.Flags().StringP("name", "n", "", "Bastion name")
  tunnelCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  tunnelCmd.Flags().String("target-resource-id", "", "Target resource ID")
  tunnelCmd.Flags().Int("resource-port", 443, "Target resource port")
  tunnelCmd.Flags().IntP("port", "p", 0, "Local port")
  tunnelCmd.MarkFlagRequired("name")
  tunnelCmd.MarkFlagRequired("resource-group")
  tunnelCmd.MarkFlagRequired("target-resource-id")
  tunnelCmd.MarkFlagRequired("port")

  cmd.AddCommand(tunnelCmd)
  return cmd
}
