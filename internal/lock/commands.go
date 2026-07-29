package lock

import (
  "github.com/spf13/cobra"
)

// newVerbCmds builds the lock verbs for a given group kind. The four azure-cli
// lock groups share one implementation and differ only in which scope flags
// each verb registers.
//
// Tasks 7-10 append newShowCmd, newDeleteCmd, newCreateCmd, and newUpdateCmd
// here as each is created.
func newVerbCmds(kind scopeKind) []*cobra.Command {
  return []*cobra.Command{
    newCreateCmd(kind),
    newDeleteCmd(kind),
    newListCmd(kind),
    newShowCmd(kind),
    newUpdateCmd(kind),
  }
}

func newGroupCmd(use, short string, kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:          use,
    Short:        short,
    SilenceUsage: true,
  }
  verbs := newVerbCmds(kind)
  // Cobra only honors SilenceUsage on the leaf command it executes and on the
  // root Execute() was called on, not on an intermediate parent. Set it on each
  // verb so a scope-validation error prints just the error, not the full usage
  // block. Without this the flag above would be silently ineffective.
  for _, v := range verbs {
    v.SilenceUsage = true
  }
  cmd.AddCommand(verbs...)
  return cmd
}

// NewLockCommand returns the root `az lock` cobra command.
func NewLockCommand() *cobra.Command {
  return newGroupCmd("lock", "Manage Azure locks", kindGeneric)
}

// NewAccountLockCommand returns `az account lock`.
func NewAccountLockCommand() *cobra.Command {
  return newGroupCmd("lock", "Manage Azure subscription level locks", kindAccount)
}

// NewGroupLockCommand returns `az group lock`.
func NewGroupLockCommand() *cobra.Command {
  return newGroupCmd("lock", "Manage Azure resource group locks", kindGroup)
}

// NewResourceLockCommand returns `az resource lock`.
func NewResourceLockCommand() *cobra.Command {
  return newGroupCmd("lock", "Manage Azure resource level locks", kindResource)
}
