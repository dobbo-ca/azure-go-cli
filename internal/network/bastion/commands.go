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

      bufferConfig := DefaultBufferConfig()
      connReadKB, _ := cmd.Flags().GetInt("conn-read-buffer")
      connWriteKB, _ := cmd.Flags().GetInt("conn-write-buffer")
      chunkReadKB, _ := cmd.Flags().GetInt("chunk-read-buffer")
      chunkWriteKB, _ := cmd.Flags().GetInt("chunk-write-buffer")

      bufferConfig.ConnReadBufferSize = connReadKB * 1024
      bufferConfig.ConnWriteBufferSize = connWriteKB * 1024
      bufferConfig.ChunkReadBufferSize = chunkReadKB * 1024
      bufferConfig.ChunkWriteBufferSize = chunkWriteKB * 1024

      return Tunnel(context.Background(), bastionName, resourceGroup, targetResourceID, resourcePort, localPort, bufferConfig)
    },
  }
  tunnelCmd.Flags().StringP("name", "n", "", "Bastion name")
  tunnelCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  tunnelCmd.Flags().String("target-resource-id", "", "Target resource ID")
  tunnelCmd.Flags().Int("resource-port", 443, "Target resource port")
  tunnelCmd.Flags().IntP("port", "p", 0, "Local port")
  tunnelCmd.Flags().Int("conn-read-buffer", 32, "Connection-level read buffer size in KB (default 32)")
  tunnelCmd.Flags().Int("conn-write-buffer", 32, "Connection-level write buffer size in KB (default 32)")
  tunnelCmd.Flags().Int("chunk-read-buffer", 8, "Streaming chunk read buffer size in KB (default 8)")
  tunnelCmd.Flags().Int("chunk-write-buffer", 8, "Streaming chunk write buffer size in KB (default 8)")
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

      bufferConfig := DefaultBufferConfig()
      connReadKB, _ := cmd.Flags().GetInt("conn-read-buffer")
      connWriteKB, _ := cmd.Flags().GetInt("conn-write-buffer")
      chunkReadKB, _ := cmd.Flags().GetInt("chunk-read-buffer")
      chunkWriteKB, _ := cmd.Flags().GetInt("chunk-write-buffer")

      bufferConfig.ConnReadBufferSize = connReadKB * 1024
      bufferConfig.ConnWriteBufferSize = connWriteKB * 1024
      bufferConfig.ChunkReadBufferSize = chunkReadKB * 1024
      bufferConfig.ChunkWriteBufferSize = chunkWriteKB * 1024

      return SSH(context.Background(), bastionName, resourceGroup, targetResourceID, authType, username, bufferConfig)
    },
  }
  sshCmd.Flags().StringP("name", "n", "", "Bastion name")
  sshCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
  sshCmd.Flags().String("target-resource-id", "", "Target VM resource ID")
  sshCmd.Flags().String("auth-type", "AAD", "Authentication type (AAD, password, ssh-key)")
  sshCmd.Flags().StringP("username", "u", "", "SSH username (Azure AD email for AAD auth)")
  sshCmd.Flags().Int("conn-read-buffer", 32, "Connection-level read buffer size in KB (default 32)")
  sshCmd.Flags().Int("conn-write-buffer", 32, "Connection-level write buffer size in KB (default 32)")
  sshCmd.Flags().Int("chunk-read-buffer", 8, "Streaming chunk read buffer size in KB (default 8)")
  sshCmd.Flags().Int("chunk-write-buffer", 8, "Streaming chunk write buffer size in KB (default 8)")
  sshCmd.MarkFlagRequired("name")
  sshCmd.MarkFlagRequired("resource-group")
  sshCmd.MarkFlagRequired("target-resource-id")

  cmd.AddCommand(tunnelCmd, sshCmd)
  return cmd
}
