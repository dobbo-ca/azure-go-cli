package quota

import (
  "context"

  "github.com/spf13/cobra"
)

func NewQuotaCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "quota",
    Short: "Manage Azure quotas and limits",
    Long:  "Commands to view and request changes to Azure quotas and service limits",
  }

  listCmd := &cobra.Command{
    Use:   "list",
    Short: "List quotas for a specific scope",
    Long:  "List all quotas for a specific resource provider and location",
    RunE: func(cmd *cobra.Command, args []string) error {
      scope, _ := cmd.Flags().GetString("scope")
      outputFormat, _ := cmd.Flags().GetString("output")
      return List(context.Background(), scope, outputFormat)
    },
  }
  listCmd.Flags().String("scope", "", "Scope for quota (e.g., subscriptions/{subscriptionId}/providers/Microsoft.Compute/locations/westeurope)")
  listCmd.Flags().StringP("output", "o", "table", "Output format: json, table")
  listCmd.MarkFlagRequired("scope")

  showCmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a specific quota",
    Long:  "Show details of a specific quota by resource name",
    RunE: func(cmd *cobra.Command, args []string) error {
      scope, _ := cmd.Flags().GetString("scope")
      resourceName, _ := cmd.Flags().GetString("resource-name")
      outputFormat, _ := cmd.Flags().GetString("output")
      return Show(context.Background(), scope, resourceName, outputFormat)
    },
  }
  showCmd.Flags().String("scope", "", "Scope for quota")
  showCmd.Flags().String("resource-name", "", "Resource name (e.g., standardDSv3Family)")
  showCmd.Flags().StringP("output", "o", "json", "Output format: json, table")
  showCmd.MarkFlagRequired("scope")
  showCmd.MarkFlagRequired("resource-name")

  requestCmd := &cobra.Command{
    Use:   "request",
    Short: "Manage quota increase requests",
    Long:  "Create and manage quota increase requests",
  }

  requestCreateCmd := &cobra.Command{
    Use:   "create",
    Short: "Create a quota increase request",
    Long:  "Submit a request to increase a quota limit",
    RunE: func(cmd *cobra.Command, args []string) error {
      scope, _ := cmd.Flags().GetString("scope")
      resourceName, _ := cmd.Flags().GetString("resource-name")
      limit, _ := cmd.Flags().GetInt32("limit")
      region, _ := cmd.Flags().GetString("region")
      return RequestCreate(context.Background(), scope, resourceName, limit, region)
    },
  }
  requestCreateCmd.Flags().String("scope", "", "Scope for quota (e.g., subscriptions/{subscriptionId}/providers/Microsoft.Compute/locations/westeurope)")
  requestCreateCmd.Flags().String("resource-name", "", "Resource name (e.g., standardDSv3Family)")
  requestCreateCmd.Flags().Int32("limit", 0, "New limit value to request")
  requestCreateCmd.Flags().String("region", "", "Region for the quota (e.g., westeurope)")
  requestCreateCmd.MarkFlagRequired("scope")
  requestCreateCmd.MarkFlagRequired("resource-name")
  requestCreateCmd.MarkFlagRequired("limit")

  requestListCmd := &cobra.Command{
    Use:   "list",
    Short: "List quota requests",
    Long:  "List all quota increase requests for a scope",
    RunE: func(cmd *cobra.Command, args []string) error {
      scope, _ := cmd.Flags().GetString("scope")
      outputFormat, _ := cmd.Flags().GetString("output")
      return RequestList(context.Background(), scope, outputFormat)
    },
  }
  requestListCmd.Flags().String("scope", "", "Scope for quota requests")
  requestListCmd.Flags().StringP("output", "o", "table", "Output format: json, table")
  requestListCmd.MarkFlagRequired("scope")

  requestCmd.AddCommand(requestCreateCmd, requestListCmd)
  cmd.AddCommand(listCmd, showCmd, requestCmd)
  return cmd
}
