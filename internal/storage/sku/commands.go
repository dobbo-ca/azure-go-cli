package sku

import (
	"context"

	"github.com/spf13/cobra"
)

func NewSKUCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sku",
		Short: "Manage storage SKUs",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available storage SKUs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return List(context.Background(), cmd)
		},
	}

	cmd.AddCommand(listCmd)

	return cmd
}
