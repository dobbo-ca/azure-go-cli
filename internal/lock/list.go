package lock

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newListCmd(kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List lock information",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runList(cmd)
    },
  }
  addScopeFlags(cmd, kind)
  addScopeShortcutFlag(cmd)
  cmd.Flags().String("filter-string", "", `A query filter to restrict the results. ARM returns locks at the given scope AND all ancestor scopes; pass "atScope()" to list only locks at this scope exactly`)
  if kind == kindGroup {
    _ = cmd.MarkFlagRequired("resource-group")
  }
  return cmd
}

func runList(cmd *cobra.Command) error {
  ctx := context.Background()
  client, err := newLocksClient(cmd)
  if err != nil {
    return err
  }
  var filter *string
  if f, _ := cmd.Flags().GetString("filter-string"); f != "" {
    filter = &f
  }

  var objs []*armlocks.ManagementLockObject

  if scope, _ := cmd.Flags().GetString("scope"); scope != "" {
    p := client.NewListByScopePager(scope, &armlocks.ManagementLocksClientListByScopeOptions{Filter: filter})
    for p.More() {
      page, err := p.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("list locks: %w", err)
      }
      objs = append(objs, page.Value...)
    }
    format, _ := cmd.Flags().GetString("output")
    return output.PrintFormatted(cmd, toLockRecords(objs), format)
  }

  s, err := resolveScope(cmd)
  if err != nil {
    return err
  }

  switch s.Level {
  case scopeResourceGroup:
    p := client.NewListAtResourceGroupLevelPager(s.ResourceGroup, &armlocks.ManagementLocksClientListAtResourceGroupLevelOptions{Filter: filter})
    for p.More() {
      page, err := p.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("list locks: %w", err)
      }
      objs = append(objs, page.Value...)
    }
  case scopeResource:
    p := client.NewListAtResourceLevelPager(s.ResourceGroup, s.Namespace, s.Parent, s.ResourceType, s.ResourceName, &armlocks.ManagementLocksClientListAtResourceLevelOptions{Filter: filter})
    for p.More() {
      page, err := p.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("list locks: %w", err)
      }
      objs = append(objs, page.Value...)
    }
  default:
    p := client.NewListAtSubscriptionLevelPager(&armlocks.ManagementLocksClientListAtSubscriptionLevelOptions{Filter: filter})
    for p.More() {
      page, err := p.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("list locks: %w", err)
      }
      objs = append(objs, page.Value...)
    }
  }

  format, _ := cmd.Flags().GetString("output")
  return output.PrintFormatted(cmd, toLockRecords(objs), format)
}
