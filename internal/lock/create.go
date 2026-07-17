package lock

import (
  "context"
  "fmt"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

// parseLockLevel accepts a lock type case-insensitively and returns the
// canonical value, matching azure-cli. NotSpecified exists in the SDK but
// azure-cli narrows it out, so reject it.
func parseLockLevel(s string) (armlocks.LockLevel, error) {
  switch {
  case strings.EqualFold(s, string(armlocks.LockLevelCanNotDelete)):
    return armlocks.LockLevelCanNotDelete, nil
  case strings.EqualFold(s, string(armlocks.LockLevelReadOnly)):
    return armlocks.LockLevelReadOnly, nil
  }
  return "", fmt.Errorf("invalid --lock-type %q (use CanNotDelete or ReadOnly)", s)
}

func newCreateCmd(kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:   "create",
    Short: "Create a lock",
    Long:  "Create a lock. Locks can exist at three different scopes: subscription, resource group and resource.",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runCreate(cmd)
    },
  }
  addScopeFlags(cmd, kind)
  cmd.Flags().StringP("name", "n", "", "Name of the lock")
  cmd.Flags().StringP("lock-type", "t", "", "The type of lock restriction. Allowed values: CanNotDelete, ReadOnly")
  cmd.Flags().String("notes", "", "Notes about this lock")
  _ = cmd.MarkFlagRequired("name")
  _ = cmd.MarkFlagRequired("lock-type")
  if kind == kindGroup {
    _ = cmd.MarkFlagRequired("resource-group")
  }
  return cmd
}

func runCreate(cmd *cobra.Command) error {
  ctx := context.Background()
  name, _ := cmd.Flags().GetString("name")
  lockType, _ := cmd.Flags().GetString("lock-type")
  level, err := parseLockLevel(lockType)
  if err != nil {
    return err
  }
  s, err := resolveScope(cmd)
  if err != nil {
    return err
  }
  client, err := newLocksClient(cmd)
  if err != nil {
    return err
  }

  params := armlocks.ManagementLockObject{
    Properties: &armlocks.ManagementLockProperties{Level: &level},
  }
  if cmd.Flags().Changed("notes") {
    notes, _ := cmd.Flags().GetString("notes")
    params.Properties.Notes = &notes
  }

  // create is really create-or-update: an existing lock with the same name at
  // the same scope is silently overwritten. azure-cli behaves the same way, and
  // deliberately runs no precheck here.
  var obj *armlocks.ManagementLockObject
  switch s.Level {
  case scopeResourceGroup:
    resp, err := client.CreateOrUpdateAtResourceGroupLevel(ctx, s.ResourceGroup, name, params, nil)
    if err != nil {
      return fmt.Errorf("create lock %s: %w", name, err)
    }
    obj = &resp.ManagementLockObject
  case scopeResource:
    resp, err := client.CreateOrUpdateAtResourceLevel(ctx, s.ResourceGroup, s.Namespace, s.Parent, s.ResourceType, s.ResourceName, name, params, nil)
    if err != nil {
      return fmt.Errorf("create lock %s: %w", name, err)
    }
    obj = &resp.ManagementLockObject
  default:
    resp, err := client.CreateOrUpdateAtSubscriptionLevel(ctx, name, params, nil)
    if err != nil {
      return fmt.Errorf("create lock %s: %w", name, err)
    }
    obj = &resp.ManagementLockObject
  }

  format, _ := cmd.Flags().GetString("output")
  return output.PrintFormatted(cmd, toLockRecord(obj), format)
}
