package lock

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newUpdateCmd(kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:   "update",
    Short: "Update a lock",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runUpdate(cmd)
    },
  }
  addScopeFlags(cmd, kind)
  addScopeShortcutFlag(cmd)
  addIDsFlag(cmd)
  cmd.Flags().StringP("name", "n", "", "Name of the lock")
  cmd.Flags().StringP("lock-type", "t", "", "The type of lock restriction. Allowed values: CanNotDelete, ReadOnly")
  cmd.Flags().String("notes", "", "Notes about this lock")
  return cmd
}

// mergeLockParams applies the read-modify-write merge: it starts from the
// existing lock's properties, then overrides level/notes with the flags the
// user actually passed (nil means "not passed, preserve").
func mergeLockParams(existing *armlocks.ManagementLockObject, newLevel *armlocks.LockLevel, newNotes *string) armlocks.ManagementLockObject {
  params := armlocks.ManagementLockObject{Properties: &armlocks.ManagementLockProperties{}}
  if existing != nil && existing.Properties != nil {
    params.Properties.Level = existing.Properties.Level
    params.Properties.Notes = existing.Properties.Notes
    params.Properties.Owners = existing.Properties.Owners
  }
  if newLevel != nil {
    params.Properties.Level = newLevel
  }
  if newNotes != nil {
    params.Properties.Notes = newNotes
  }
  return params
}

func runUpdate(cmd *cobra.Command) error {
  ctx := context.Background()

  // A read-modify-write partial merge: only flags the user actually passed are
  // applied. Test Changed(), not emptiness — `--notes ""` clears the notes
  // while omitting --notes preserves them, and those must stay distinct.
  var newLevel *armlocks.LockLevel
  if cmd.Flags().Changed("lock-type") {
    lockType, _ := cmd.Flags().GetString("lock-type")
    lvl, err := parseLockLevel(lockType)
    if err != nil {
      return err
    }
    newLevel = &lvl
  }
  var newNotes *string
  if cmd.Flags().Changed("notes") {
    notes, _ := cmd.Flags().GetString("notes")
    newNotes = &notes
  }

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
    params := mergeLockParams(&resp.ManagementLockObject, newLevel, newNotes)
    if params.Properties.Level == nil {
      return fmt.Errorf("lock %s has no level; --lock-type is required", name)
    }
    upd, err := client.CreateOrUpdateByScope(ctx, scope, name, params, nil)
    if err != nil {
      return fmt.Errorf("update lock %s: %w", name, err)
    }
    format, _ := cmd.Flags().GetString("output")
    return output.PrintFormatted(cmd, toLockRecord(&upd.ManagementLockObject), format)
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
    existing, err := getLock(ctx, client, t)
    if err != nil {
      return err
    }
    params := mergeLockParams(existing, newLevel, newNotes)
    if params.Properties.Level == nil {
      return fmt.Errorf("lock %s has no level; --lock-type is required", t.LockName)
    }

    var obj *armlocks.ManagementLockObject
    switch t.Scope.Level {
    case scopeResourceGroup:
      resp, err := client.CreateOrUpdateAtResourceGroupLevel(ctx, t.Scope.ResourceGroup, t.LockName, params, nil)
      if err != nil {
        return fmt.Errorf("update lock %s: %w", t.LockName, err)
      }
      obj = &resp.ManagementLockObject
    case scopeResource:
      resp, err := client.CreateOrUpdateAtResourceLevel(ctx, t.Scope.ResourceGroup, t.Scope.Namespace, t.Scope.Parent, t.Scope.ResourceType, t.Scope.ResourceName, t.LockName, params, nil)
      if err != nil {
        return fmt.Errorf("update lock %s: %w", t.LockName, err)
      }
      obj = &resp.ManagementLockObject
    default:
      resp, err := client.CreateOrUpdateAtSubscriptionLevel(ctx, t.LockName, params, nil)
      if err != nil {
        return fmt.Errorf("update lock %s: %w", t.LockName, err)
      }
      obj = &resp.ManagementLockObject
    }
    results = append(results, toLockRecord(obj))
  }

  format, _ := cmd.Flags().GetString("output")
  if len(results) == 1 {
    return output.PrintFormatted(cmd, results[0], format)
  }
  return output.PrintFormatted(cmd, results, format)
}
