package lock

import (
  "context"
  "fmt"

  "github.com/spf13/cobra"
)

func newDeleteCmd(kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a lock",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runDelete(cmd)
    },
  }
  addScopeFlags(cmd, kind)
  addIDsFlag(cmd)
  cmd.Flags().StringP("name", "n", "", "Name of the lock")
  return cmd
}

func runDelete(cmd *cobra.Command) error {
  ctx := context.Background()
  targets, err := resolveTargets(cmd)
  if err != nil {
    return err
  }
  client, err := newLocksClient(cmd)
  if err != nil {
    return err
  }
  for _, t := range targets {
    if err := runPrecheck(ctx, cmd, t.Scope, t.LockName); err != nil {
      return err
    }
    switch t.Scope.Level {
    case scopeResourceGroup:
      _, err = client.DeleteAtResourceGroupLevel(ctx, t.Scope.ResourceGroup, t.LockName, nil)
    case scopeResource:
      _, err = client.DeleteAtResourceLevel(ctx, t.Scope.ResourceGroup, t.Scope.Namespace, t.Scope.Parent, t.Scope.ResourceType, t.Scope.ResourceName, t.LockName, nil)
    default:
      _, err = client.DeleteAtSubscriptionLevel(ctx, t.LockName, nil)
    }
    if err != nil {
      return fmt.Errorf("delete lock %s: %w", t.LockName, err)
    }
  }
  // azure-cli's delete returns no body.
  return nil
}
