package resource

import (
  "context"
  "fmt"

  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a resource",
    RunE:  runDelete,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  return cmd
}

func runDelete(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  client, cred, sub, err := newGenericClient(cmd)
  if err != nil {
    return err
  }
  explicit, _ := cmd.Flags().GetString("api-version")
  preview, _ := cmd.Flags().GetBool("latest-include-preview")

  for _, id := range ids {
    _, _, namespace, types, _, perr := ParseResourceID(id)
    if perr != nil {
      return perr
    }
    apiVer, err := azure.ResolveAPIVersion(ctx, cred, sub, namespace, joinTypes(types), explicit, preview)
    if err != nil {
      return err
    }
    poller, err := client.BeginDeleteByID(ctx, id, apiVer, nil)
    if err != nil {
      return fmt.Errorf("delete %s: %w", id, err)
    }
    if _, err := poller.PollUntilDone(ctx, nil); err != nil {
      return fmt.Errorf("delete %s: %w", id, err)
    }
    fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", id)
  }
  return nil
}
