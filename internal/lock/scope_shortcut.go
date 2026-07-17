package lock

import (
  "fmt"

  "github.com/spf13/cobra"
)

// addScopeShortcutFlag registers --scope.
//
// This has no azure-cli equivalent. The SDK's *ByScope family collapses all
// three scope levels into one call and additionally reaches management groups,
// which az lock cannot. It is purely additive: --scope bypasses the scope
// validator entirely, and every azure-cli invocation keeps working untouched.
func addScopeShortcutFlag(cmd *cobra.Command) {
  cmd.Flags().String("scope", "", "Full scope to lock (e.g. /subscriptions/{id}, or /providers/Microsoft.Management/managementGroups/{id}). Bypasses the other scope flags")
}

// scopeShortcutName validates the flags that accompany --scope on the verbs
// that also accept --ids, and returns the lock name. --scope selects a single
// lock by name, so --ids is not allowed alongside it and --name is required.
func scopeShortcutName(cmd *cobra.Command) (string, error) {
  if ids, _ := cmd.Flags().GetStringSlice("ids"); len(ids) > 0 {
    return "", fmt.Errorf("cannot mix --scope with --ids")
  }
  name, _ := cmd.Flags().GetString("name")
  if name == "" {
    return "", fmt.Errorf("--name is required with --scope")
  }
  return name, nil
}
