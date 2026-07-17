package lock

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newShowCmd(kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:   "show",
    Short: "Show the properties of a lock",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runShow(cmd)
    },
  }
  addScopeFlags(cmd, kind)
  addScopeShortcutFlag(cmd)
  addIDsFlag(cmd)
  cmd.Flags().StringP("name", "n", "", "Name of the lock")
  return cmd
}

func runShow(cmd *cobra.Command) error {
  ctx := context.Background()

  if scope, _ := cmd.Flags().GetString("scope"); scope != "" {
    name, err := scopeShortcutName(cmd)
    if err != nil {
      return err
    }
    client, err := newLocksClient(cmd)
    if err != nil {
      return err
    }
    resp, err := client.GetByScope(ctx, scope, name, nil)
    if err != nil {
      return fmt.Errorf("get lock %s: %w", name, err)
    }
    format, _ := cmd.Flags().GetString("output")
    return output.PrintFormatted(cmd, toLockRecord(&resp.ManagementLockObject), format)
  }

  targets, err := resolveTargets(cmd)
  if err != nil {
    return err
  }
  client, err := newLocksClient(cmd)
  if err != nil {
    return err
  }

  results := make([]lockRecord, 0, len(targets))
  for _, t := range targets {
    if err := runPrecheck(ctx, cmd, t.Scope, t.LockName); err != nil {
      return err
    }
    obj, err := getLock(ctx, client, t)
    if err != nil {
      return err
    }
    results = append(results, toLockRecord(obj))
  }

  format, _ := cmd.Flags().GetString("output")
  // azure-cli returns a bare object for one id and an array for several.
  if len(results) == 1 {
    return output.PrintFormatted(cmd, results[0], format)
  }
  return output.PrintFormatted(cmd, results, format)
}

func getLock(ctx context.Context, client *armlocks.ManagementLocksClient, t lockTarget) (*armlocks.ManagementLockObject, error) {
  switch t.Scope.Level {
  case scopeResourceGroup:
    resp, err := client.GetAtResourceGroupLevel(ctx, t.Scope.ResourceGroup, t.LockName, nil)
    if err != nil {
      return nil, fmt.Errorf("get lock %s: %w", t.LockName, err)
    }
    return &resp.ManagementLockObject, nil
  case scopeResource:
    resp, err := client.GetAtResourceLevel(ctx, t.Scope.ResourceGroup, t.Scope.Namespace, t.Scope.Parent, t.Scope.ResourceType, t.Scope.ResourceName, t.LockName, nil)
    if err != nil {
      return nil, fmt.Errorf("get lock %s: %w", t.LockName, err)
    }
    return &resp.ManagementLockObject, nil
  default:
    resp, err := client.GetAtSubscriptionLevel(ctx, t.LockName, nil)
    if err != nil {
      return nil, fmt.Errorf("get lock %s: %w", t.LockName, err)
    }
    return &resp.ManagementLockObject, nil
  }
}
