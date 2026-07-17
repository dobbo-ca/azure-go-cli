package lock

import (
  "context"
  "fmt"

  "github.com/spf13/cobra"
)

// lockTarget is a single resolved lock operation target.
type lockTarget struct {
  Scope    lockScope
  LockName string
}

func addIDsFlag(cmd *cobra.Command) {
  cmd.Flags().StringSlice("ids", nil, "One or more lock IDs (space- or comma-separated). If supplied, no other resource arguments should be specified")
}

// resolveTargets turns --ids or the scope flags into one target per lock.
//
// azure-cli logs an error and exits 0 on an unparseable ID, silently swallowing
// typos. We return a real error instead. Spec divergence 3.
func resolveTargets(cmd *cobra.Command) ([]lockTarget, error) {
  ids, _ := cmd.Flags().GetStringSlice("ids")
  name, _ := cmd.Flags().GetString("name")

  if len(ids) > 0 {
    if name != "" {
      return nil, fmt.Errorf("cannot mix --ids with --name")
    }
    targets := make([]lockTarget, 0, len(ids))
    for _, id := range ids {
      parts, err := parseLockID(id)
      if err != nil {
        return nil, err
      }
      targets = append(targets, lockTarget{Scope: scopeFromIDParts(parts), LockName: parts.LockName})
    }
    return targets, nil
  }

  if name == "" {
    return nil, fmt.Errorf("--name is required when --ids is not given")
  }
  s, err := resolveScope(cmd)
  if err != nil {
    return nil, err
  }
  return []lockTarget{{Scope: s, LockName: name}}, nil
}

func scopeFromIDParts(p lockIDParts) lockScope {
  s := lockScope{
    ResourceGroup: p.ResourceGroup,
    Namespace:     p.Namespace,
    Parent:        p.Parent,
    ResourceType:  p.ResourceType,
    ResourceName:  p.ResourceName,
  }
  switch {
  case p.ResourceName != "":
    s.Level = scopeResource
  case p.ResourceGroup != "":
    s.Level = scopeResourceGroup
  default:
    s.Level = scopeSubscription
  }
  return s
}

// runPrecheck ports the listing half of _validate_lock_params_match_lock: list
// locks subscription-wide, and if exactly one matches by name, verify the
// user's scope flags against it. If the count is not exactly one, azure-cli
// performs no validation at all — match that.
//
// This costs a subscription-wide list on every show/delete/update and needs
// subscription-wide lock-read permission. That trade was made deliberately; see
// the spec.
func runPrecheck(ctx context.Context, cmd *cobra.Command, want lockScope, lockName string) error {
  client, err := newLocksClient(cmd)
  if err != nil {
    return err
  }
  var matches []lockIDParts
  p := client.NewListAtSubscriptionLevelPager(nil)
  for p.More() {
    page, err := p.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("list locks: %w", err)
    }
    for _, o := range page.Value {
      if o == nil || o.Name == nil || *o.Name != lockName || o.ID == nil {
        continue
      }
      parts, err := parseLockID(*o.ID)
      if err != nil {
        continue
      }
      matches = append(matches, parts)
    }
  }
  if len(matches) != 1 {
    return nil
  }
  return validateScopeMatchesLock(want, matches[0], lockName)
}
