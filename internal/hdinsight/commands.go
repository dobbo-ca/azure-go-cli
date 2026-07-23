package hdinsight

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/hdinsight/application"
	"github.com/cdobbyn/azure-go-cli/internal/hdinsight/azuremonitor"
	"github.com/cdobbyn/azure-go-cli/internal/hdinsight/credentials"
	"github.com/cdobbyn/azure-go-cli/internal/hdinsight/host"
	"github.com/cdobbyn/azure-go-cli/internal/hdinsight/monitor"
	"github.com/cdobbyn/azure-go-cli/internal/hdinsight/scriptaction"
	"github.com/spf13/cobra"
)

func NewHDInsightCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hdinsight",
		Short: "Manage Azure HDInsight clusters",
		Long:  "Commands to manage Azure HDInsight clusters",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List HDInsight clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, rg)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of an HDInsight cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, rg, name)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Cluster name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an HDInsight cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), cmd, rg, name, noWait)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Cluster name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("resource-group")

	resizeCmd := &cobra.Command{
		Use:   "resize",
		Short: "Resize the number of worker nodes in an HDInsight cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			role, _ := cmd.Flags().GetString("role-name")
			count, _ := cmd.Flags().GetInt32("target-instance-count")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Resize(context.Background(), cmd, rg, name, role, count, noWait)
		},
	}
	resizeCmd.Flags().StringP("name", "n", "", "Cluster name")
	resizeCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	resizeCmd.Flags().String("role-name", "workernode", "Role to resize")
	resizeCmd.Flags().Int32("target-instance-count", 0, "Target instance count")
	resizeCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	resizeCmd.MarkFlagRequired("name")
	resizeCmd.MarkFlagRequired("resource-group")
	resizeCmd.MarkFlagRequired("target-instance-count")

	rotateKeyCmd := &cobra.Command{
		Use:   "rotate-disk-encryption-key",
		Short: "Rotate the disk encryption key of an HDInsight cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			vaultURI, _ := cmd.Flags().GetString("vault-uri")
			keyName, _ := cmd.Flags().GetString("key-name")
			keyVersion, _ := cmd.Flags().GetString("key-version")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return RotateDiskEncryptionKey(context.Background(), cmd, rg, name, vaultURI, keyName, keyVersion, noWait)
		},
	}
	rotateKeyCmd.Flags().StringP("name", "n", "", "Cluster name")
	rotateKeyCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	rotateKeyCmd.Flags().String("vault-uri", "", "Key vault URI, e.g. https://myvault.vault.azure.net")
	rotateKeyCmd.Flags().String("key-name", "", "Key name used for disk encryption")
	rotateKeyCmd.Flags().String("key-version", "", "Key version used for disk encryption")
	rotateKeyCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	rotateKeyCmd.MarkFlagRequired("name")
	rotateKeyCmd.MarkFlagRequired("resource-group")
	rotateKeyCmd.MarkFlagRequired("vault-uri")
	rotateKeyCmd.MarkFlagRequired("key-name")
	rotateKeyCmd.MarkFlagRequired("key-version")

	listUsageCmd := &cobra.Command{
		Use:   "list-usage",
		Short: "List regional usage (quota) for HDInsight",
		RunE: func(cmd *cobra.Command, args []string) error {
			location, _ := cmd.Flags().GetString("location")
			return ListUsage(context.Background(), cmd, location)
		},
	}
	listUsageCmd.Flags().StringP("location", "l", "", "Azure region")
	listUsageCmd.MarkFlagRequired("location")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update the tags of an HDInsight cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			tags, _ := cmd.Flags().GetStringSlice("tags")
			return Update(context.Background(), cmd, rg, name, tags)
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "Cluster name")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().StringSlice("tags", nil, "Resource tags as key=value pairs")
	updateCmd.MarkFlagRequired("name")
	updateCmd.MarkFlagRequired("resource-group")

	waitCmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait for an HDInsight cluster to reach a condition",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			deleted, _ := cmd.Flags().GetBool("deleted")
			exists, _ := cmd.Flags().GetBool("exists")
			interval, _ := cmd.Flags().GetInt("interval")
			timeout, _ := cmd.Flags().GetInt("timeout")
			return Wait(context.Background(), cmd, rg, name, deleted, exists, interval, timeout)
		},
	}
	waitCmd.Flags().StringP("name", "n", "", "Cluster name")
	waitCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	waitCmd.Flags().Bool("created", false, "Wait until succeeded (default behavior)")
	waitCmd.Flags().Bool("deleted", false, "Wait until deleted")
	waitCmd.Flags().Bool("exists", false, "Wait until the cluster exists")
	waitCmd.Flags().Int("interval", 30, "Polling interval in seconds")
	waitCmd.Flags().Int("timeout", 3600, "Maximum wait time in seconds")
	waitCmd.MarkFlagRequired("name")
	waitCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(
		listCmd, showCmd, deleteCmd, resizeCmd, rotateKeyCmd, listUsageCmd, updateCmd, waitCmd,
		application.NewApplicationCommand(),
		azuremonitor.NewAzureMonitorCommand(),
		credentials.NewCredentialsCommand(),
		host.NewHostCommand(),
		monitor.NewMonitorCommand(),
		scriptaction.NewScriptActionCommand(),
	)
	return cmd
}
