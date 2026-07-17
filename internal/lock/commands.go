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
    newListCmd(kind),
  }
}

func newGroupCmd(use, short string, kind scopeKind) *cobra.Command {
  cmd := &cobra.Command{
    Use:          use,
    Short:        short,
    SilenceUsage: true,
  }
  cmd.AddCommand(newVerbCmds(kind)...)
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
