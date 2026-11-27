package blob

import (
	"context"

	"github.com/spf13/cobra"
)

func NewBlobCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blob",
		Short: "Manage blob storage",
		Long:  "Commands to manage blobs in Azure storage containers",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List blobs in a container",
		RunE: func(cmd *cobra.Command, args []string) error {
			containerName, _ := cmd.Flags().GetString("container-name")
			return List(context.Background(), cmd, containerName)
		},
	}
	addAuthFlags(listCmd)
	listCmd.Flags().StringP("container-name", "c", "", "Container name")
	listCmd.MarkFlagRequired("container-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a blob",
		RunE: func(cmd *cobra.Command, args []string) error {
			containerName, _ := cmd.Flags().GetString("container-name")
			blobName, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, containerName, blobName)
		},
	}
	addAuthFlags(showCmd)
	showCmd.Flags().StringP("container-name", "c", "", "Container name")
	showCmd.Flags().StringP("name", "n", "", "Blob name")
	showCmd.MarkFlagRequired("container-name")
	showCmd.MarkFlagRequired("name")

	uploadCmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload a file to blob storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			containerName, _ := cmd.Flags().GetString("container-name")
			blobName, _ := cmd.Flags().GetString("name")
			filePath, _ := cmd.Flags().GetString("file")
			overwrite, _ := cmd.Flags().GetBool("overwrite")
			return Upload(context.Background(), cmd, containerName, blobName, filePath, overwrite)
		},
	}
	addAuthFlags(uploadCmd)
	uploadCmd.Flags().StringP("container-name", "c", "", "Container name")
	uploadCmd.Flags().StringP("name", "n", "", "Blob name (defaults to filename if not specified)")
	uploadCmd.Flags().StringP("file", "f", "", "Path to file to upload")
	uploadCmd.Flags().Bool("overwrite", false, "Overwrite existing blob")
	uploadCmd.MarkFlagRequired("container-name")
	uploadCmd.MarkFlagRequired("file")

	downloadCmd := &cobra.Command{
		Use:   "download",
		Short: "Download a blob from storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			containerName, _ := cmd.Flags().GetString("container-name")
			blobName, _ := cmd.Flags().GetString("name")
			filePath, _ := cmd.Flags().GetString("file")
			return Download(context.Background(), cmd, containerName, blobName, filePath)
		},
	}
	addAuthFlags(downloadCmd)
	downloadCmd.Flags().StringP("container-name", "c", "", "Container name")
	downloadCmd.Flags().StringP("name", "n", "", "Blob name")
	downloadCmd.Flags().StringP("file", "f", "", "Path where to save the downloaded file")
	downloadCmd.MarkFlagRequired("container-name")
	downloadCmd.MarkFlagRequired("name")
	downloadCmd.MarkFlagRequired("file")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a blob",
		RunE: func(cmd *cobra.Command, args []string) error {
			containerName, _ := cmd.Flags().GetString("container-name")
			blobName, _ := cmd.Flags().GetString("name")
			return Delete(context.Background(), cmd, containerName, blobName)
		},
	}
	addAuthFlags(deleteCmd)
	deleteCmd.Flags().StringP("container-name", "c", "", "Container name")
	deleteCmd.Flags().StringP("name", "n", "", "Blob name")
	deleteCmd.MarkFlagRequired("container-name")
	deleteCmd.MarkFlagRequired("name")

	cmd.AddCommand(listCmd, showCmd, uploadCmd, downloadCmd, deleteCmd, NewGenerateSASCommand())
	return cmd
}
