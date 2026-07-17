package lock

import (
  "fmt"
  "strings"
)

// validateScopeMatchesLock ports azure-cli's _validate_lock_params_match_lock.
//
// It compares the scope the user asked for against the scope of the single lock
// that matched by name, turning what would otherwise be a bare 404 into a
// message naming the offending flag. An empty user flag never conflicts.
//
// The case-sensitivity asymmetry is azure-cli's, reproduced deliberately:
// resource group and namespace compare case-insensitively, everything else
// case-sensitively.
func validateScopeMatchesLock(want lockScope, got lockIDParts, lockName string) error {
  if want.ResourceGroup != "" && !strings.EqualFold(want.ResourceGroup, got.ResourceGroup) {
    return fmt.Errorf("unexpected --resource-group for lock %s, expected %s", lockName, got.ResourceGroup)
  }
  if want.Namespace != "" && !strings.EqualFold(want.Namespace, got.Namespace) {
    return fmt.Errorf("unexpected --namespace for lock %s, expected %s", lockName, got.Namespace)
  }
  if want.ResourceType != "" && want.ResourceType != got.ResourceType {
    return fmt.Errorf("unexpected --resource-type for lock %s, expected %s", lockName, got.ResourceType)
  }
  if want.ResourceName != "" && want.ResourceName != got.ResourceName {
    return fmt.Errorf("unexpected --resource for lock %s, expected %s", lockName, got.ResourceName)
  }
  if want.Parent != "" && want.Parent != got.Parent {
    return fmt.Errorf("unexpected --parent for lock %s, expected %s", lockName, got.Parent)
  }
  return nil
}
