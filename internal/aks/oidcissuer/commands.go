package oidcissuer

import (
	"context"

	"github.com/spf13/cobra"
)

func NewOIDCIssuerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "oidc-issuer",
		Short: "Manage the OIDC issuer of an AKS cluster",
	}

	rotateCmd := &cobra.Command{
		Use:   "rotate-signing-keys",
		Short: "Rotate the OIDC issuer service account signing keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return RotateSigningKeys(context.Background(), cmd, name, resourceGroup, noWait)
		},
	}
	rotateCmd.Flags().StringP("name", "n", "", "AKS cluster name")
	rotateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	rotateCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	rotateCmd.MarkFlagRequired("name")
	rotateCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(rotateCmd)
	return cmd
}
