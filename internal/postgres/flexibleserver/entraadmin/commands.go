package entraadmin

import (
	"context"

	"github.com/spf13/cobra"
)

func NewMicrosoftEntraAdminCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "microsoft-entra-admin",
		Short: "Manage Microsoft Entra (Azure AD) administrators for a PostgreSQL flexible server",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List Microsoft Entra administrators",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			return List(context.Background(), cmd, rg, server)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("server-name", "", "Flexible server name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("server-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a Microsoft Entra administrator",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			objectID, _ := cmd.Flags().GetString("object-id")
			return Show(context.Background(), cmd, rg, server, objectID)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("server-name", "", "Flexible server name")
	showCmd.Flags().String("object-id", "", "Object ID (GUID) of the administrator")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("server-name")
	showCmd.MarkFlagRequired("object-id")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Add a Microsoft Entra administrator",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			objectID, _ := cmd.Flags().GetString("object-id")
			displayName, _ := cmd.Flags().GetString("display-name")
			principalType, _ := cmd.Flags().GetString("type")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Create(context.Background(), cmd, rg, server, objectID, displayName, principalType, noWait)
		},
	}
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().String("server-name", "", "Flexible server name")
	createCmd.Flags().String("object-id", "", "Object ID (GUID) of the administrator")
	createCmd.Flags().String("display-name", "", "Display name (principal name) of the administrator")
	createCmd.Flags().String("type", "", "Principal type: User, Group, or ServicePrincipal")
	createCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("server-name")
	createCmd.MarkFlagRequired("object-id")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Remove a Microsoft Entra administrator",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			objectID, _ := cmd.Flags().GetString("object-id")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), cmd, rg, server, objectID, noWait)
		},
	}
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().String("server-name", "", "Flexible server name")
	deleteCmd.Flags().String("object-id", "", "Object ID (GUID) of the administrator")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("server-name")
	deleteCmd.MarkFlagRequired("object-id")

	cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
	return cmd
}
