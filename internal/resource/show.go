package resource

import (
  "context"
  "fmt"

  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "show",
    Short: "Get details of a resource",
    RunE:  runShow,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  return cmd
}

func runShow(cmd *cobra.Command, args []string) error {
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

  results := make([]interface{}, 0, len(ids))
  for _, id := range ids {
    _, _, namespace, types, _, perr := ParseResourceID(id)
    if perr != nil {
      return perr
    }
    rt := joinTypes(types)
    apiVer, err := azure.ResolveAPIVersion(ctx, cred, sub, namespace, rt, explicit, preview)
    if err != nil {
      return err
    }
    resp, err := client.GetByID(ctx, id, apiVer, nil)
    if err != nil {
      return fmt.Errorf("get %s: %w", id, err)
    }
    results = append(results, resp.GenericResource)
  }
  if len(results) == 1 {
    return output.PrintJSON(cmd, results[0])
  }
  return output.PrintJSON(cmd, results)
}

// joinTypes turns ["virtualNetworks","subnets"] into "virtualNetworks/subnets".
func joinTypes(types []string) string {
  s := ""
  for i, t := range types {
    if i > 0 {
      s += "/"
    }
    s += t
  }
  return s
}
