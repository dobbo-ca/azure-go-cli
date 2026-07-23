package command

import (
	"context"

	"github.com/spf13/cobra"
)

func NewCommandCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "command",
		Short: "Run commands against an AKS cluster through the managed run-command API",
	}

	invokeCmd := &cobra.Command{
		Use:   "invoke",
		Short: "Run a shell command in the cluster's context",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			command, _ := cmd.Flags().GetString("command")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Invoke(context.Background(), cmd, name, rg, command, noWait)
		},
	}
	invokeCmd.Flags().StringP("name", "n", "", "AKS cluster name")
	invokeCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	invokeCmd.Flags().String("command", "", "Command to run")
	invokeCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	invokeCmd.MarkFlagRequired("name")
	invokeCmd.MarkFlagRequired("resource-group")
	invokeCmd.MarkFlagRequired("command")

	resultCmd := &cobra.Command{
		Use:   "result",
		Short: "Fetch the result of a previously invoked command",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			commandID, _ := cmd.Flags().GetString("command-id")
			return Result(context.Background(), cmd, name, rg, commandID)
		},
	}
	resultCmd.Flags().StringP("name", "n", "", "AKS cluster name")
	resultCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	resultCmd.Flags().String("command-id", "", "Id of the command")
	resultCmd.MarkFlagRequired("name")
	resultCmd.MarkFlagRequired("resource-group")
	resultCmd.MarkFlagRequired("command-id")

	cmd.AddCommand(invokeCmd, resultCmd)
	return cmd
}
