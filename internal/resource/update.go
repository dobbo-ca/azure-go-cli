package resource

import (
  "context"
  "encoding/json"
  "fmt"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/genericupdate"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "update",
    Short: "Update a resource generically via --set/--add/--remove",
    RunE:  runUpdate,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().StringArray("set", nil, "Set a property: path=value (repeatable)")
  cmd.Flags().StringArray("add", nil, "Append to a list property: path JSON_VALUE (repeatable)")
  cmd.Flags().StringArray("remove", nil, "Remove a key or list element: path [INDEX] (repeatable)")
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  return cmd
}

func runUpdate(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  if len(ids) != 1 {
    return fmt.Errorf("update operates on a single resource")
  }
  id := ids[0]

  setOps, _ := cmd.Flags().GetStringArray("set")
  addOps, _ := cmd.Flags().GetStringArray("add")
  removeOps, _ := cmd.Flags().GetStringArray("remove")

  ops, err := parseUpdateOps(setOps, addOps, removeOps)
  if err != nil {
    return err
  }
  if len(ops) == 0 {
    return fmt.Errorf("at least one --set/--add/--remove is required")
  }

  client, cred, sub, err := newGenericClient(cmd)
  if err != nil {
    return err
  }
  _, _, namespace, types, _, perr := ParseResourceID(id)
  if perr != nil {
    return perr
  }
  explicit, _ := cmd.Flags().GetString("api-version")
  preview, _ := cmd.Flags().GetBool("latest-include-preview")
  apiVer, err := azure.ResolveAPIVersion(ctx, cred, sub, namespace, joinTypes(types), explicit, preview)
  if err != nil {
    return err
  }

  // GET the current state, mutate, PUT it back via UpdateByID (PATCH semantics).
  resp, err := client.GetByID(ctx, id, apiVer, nil)
  if err != nil {
    return fmt.Errorf("get %s: %w", id, err)
  }

  body, err := json.Marshal(resp.GenericResource)
  if err != nil {
    return err
  }
  var obj map[string]interface{}
  if err := json.Unmarshal(body, &obj); err != nil {
    return err
  }
  if err := genericupdate.Apply(obj, ops); err != nil {
    return err
  }

  raw, _ := json.Marshal(obj)
  var updated armresources.GenericResource
  if err := json.Unmarshal(raw, &updated); err != nil {
    return err
  }

  poller, err := client.BeginUpdateByID(ctx, id, apiVer, updated, nil)
  if err != nil {
    return fmt.Errorf("update %s: %w", id, err)
  }
  out, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("update %s: %w", id, err)
  }
  return output.PrintJSON(cmd, out.GenericResource)
}

func parseUpdateOps(setOps, addOps, removeOps []string) ([]genericupdate.Op, error) {
  out := []genericupdate.Op{}
  for _, s := range setOps {
    eq := strings.Index(s, "=")
    if eq == -1 {
      return nil, fmt.Errorf("--set %q: expected path=value", s)
    }
    out = append(out, genericupdate.Op{Kind: genericupdate.Set, Path: s[:eq], Value: s[eq+1:]})
  }
  for _, a := range addOps {
    sp := strings.IndexAny(a, " \t")
    if sp == -1 {
      return nil, fmt.Errorf("--add %q: expected 'path JSON_VALUE'", a)
    }
    out = append(out, genericupdate.Op{Kind: genericupdate.Add, Path: a[:sp], Value: strings.TrimSpace(a[sp+1:])})
  }
  for _, r := range removeOps {
    sp := strings.IndexAny(r, " \t")
    if sp == -1 {
      out = append(out, genericupdate.Op{Kind: genericupdate.Remove, Path: r})
      continue
    }
    out = append(out, genericupdate.Op{Kind: genericupdate.Remove, Path: r[:sp], Value: strings.TrimSpace(r[sp+1:])})
  }
  return out, nil
}
