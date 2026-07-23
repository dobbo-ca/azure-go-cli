package lock

import (
	"context"

	"github.com/spf13/cobra"
)

// NewLockCommand builds the `az account lock` command group for
// subscription-scoped management locks (Microsoft.Authorization/locks).
func NewLockCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Manage Azure subscription-level locks",
		Long:  "Commands to manage management locks at the subscription scope",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List subscription-level locks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return List(context.Background(), cmd)
		},
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a subscription-level lock",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, name)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Lock name")
	showCmd.MarkFlagRequired("name")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a subscription-level lock",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			lockType, _ := cmd.Flags().GetString("lock-type")
			notes, _ := cmd.Flags().GetString("notes")
			return Create(context.Background(), cmd, name, lockType, notes)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Lock name")
	createCmd.Flags().StringP("lock-type", "t", "CanNotDelete", "Lock level: CanNotDelete or ReadOnly")
	createCmd.Flags().String("notes", "", "Notes about the lock (max 512 characters)")
	createCmd.MarkFlagRequired("name")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a subscription-level lock",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			lockType, _ := cmd.Flags().GetString("lock-type")
			notes, _ := cmd.Flags().GetString("notes")
			return Update(context.Background(), cmd, name, lockType, notes)
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "Lock name")
	updateCmd.Flags().StringP("lock-type", "t", "", "Lock level: CanNotDelete or ReadOnly")
	updateCmd.Flags().String("notes", "", "Notes about the lock (max 512 characters)")
	updateCmd.MarkFlagRequired("name")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a subscription-level lock",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), name)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Lock name")
	deleteCmd.MarkFlagRequired("name")

	cmd.AddCommand(listCmd, showCmd, createCmd, updateCmd, deleteCmd)
	return cmd
}
