package identity

import (
	"context"

	"github.com/spf13/cobra"
)

func NewIdentityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "identity",
		Short: "Manage VMSS managed identities",
	}

	assignCmd := &cobra.Command{
		Use:   "assign",
		Short: "Assign user-assigned managed identities to a scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			identities, _ := cmd.Flags().GetStringSlice("identities")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Assign(context.Background(), cmd, rg, name, identities, noWait)
		},
	}
	assignCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	assignCmd.Flags().StringP("name", "n", "", "Scale set name")
	assignCmd.Flags().StringSlice("identities", nil, "User-assigned identity resource IDs")
	assignCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	assignCmd.MarkFlagRequired("resource-group")
	assignCmd.MarkFlagRequired("name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show the managed identity of a scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, rg, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().StringP("name", "n", "", "Scale set name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("name")

	cmd.AddCommand(assignCmd, showCmd)
	return cmd
}
