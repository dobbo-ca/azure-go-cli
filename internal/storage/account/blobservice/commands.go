package blobservice

import (
	"context"

	"github.com/spf13/cobra"
)

func NewBlobServicePropertiesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blob-service-properties",
		Short: "Manage the blob service properties of a storage account",
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show the blob service properties of a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, account, resourceGroup)
		},
	}
	showCmd.Flags().StringP("account-name", "", "", "Storage account name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("account-name")
	showCmd.MarkFlagRequired("resource-group")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update the blob service properties of a storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, _ := cmd.Flags().GetString("account-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")

			var enableDeleteRetention *bool
			if cmd.Flags().Changed("enable-delete-retention") {
				v, _ := cmd.Flags().GetBool("enable-delete-retention")
				enableDeleteRetention = &v
			}
			var deleteRetentionDays *int32
			if cmd.Flags().Changed("delete-retention-days") {
				v, _ := cmd.Flags().GetInt32("delete-retention-days")
				deleteRetentionDays = &v
			}
			var enableVersioning *bool
			if cmd.Flags().Changed("enable-versioning") {
				v, _ := cmd.Flags().GetBool("enable-versioning")
				enableVersioning = &v
			}
			var enableChangeFeed *bool
			if cmd.Flags().Changed("enable-change-feed") {
				v, _ := cmd.Flags().GetBool("enable-change-feed")
				enableChangeFeed = &v
			}
			var enableContainerDeleteRetention *bool
			if cmd.Flags().Changed("enable-container-delete-retention") {
				v, _ := cmd.Flags().GetBool("enable-container-delete-retention")
				enableContainerDeleteRetention = &v
			}
			var containerDeleteRetentionDays *int32
			if cmd.Flags().Changed("container-delete-retention-days") {
				v, _ := cmd.Flags().GetInt32("container-delete-retention-days")
				containerDeleteRetentionDays = &v
			}

			return Update(context.Background(), cmd, account, resourceGroup, enableDeleteRetention, deleteRetentionDays, enableVersioning, enableChangeFeed, enableContainerDeleteRetention, containerDeleteRetentionDays)
		},
	}
	updateCmd.Flags().StringP("account-name", "", "", "Storage account name")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().Bool("enable-delete-retention", false, "Enable blob soft delete (delete retention policy)")
	updateCmd.Flags().Int32("delete-retention-days", 0, "Number of days to retain soft deleted blobs")
	updateCmd.Flags().Bool("enable-versioning", false, "Enable blob versioning")
	updateCmd.Flags().Bool("enable-change-feed", false, "Enable change feed event logging")
	updateCmd.Flags().Bool("enable-container-delete-retention", false, "Enable container soft delete (delete retention policy)")
	updateCmd.Flags().Int32("container-delete-retention-days", 0, "Number of days to retain soft deleted containers")
	updateCmd.MarkFlagRequired("account-name")
	updateCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(showCmd, updateCmd)

	return cmd
}
