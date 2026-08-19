// Package devops implements `az devops`.
package devops

import "github.com/spf13/cobra"

// newCoreCommands returns the four `devops` commands that are direct
// siblings under the "devops" root — login, logout, configure, invoke —
// rather than a single nested group. Another phase attaches these to the
// shared "devops" root command alongside project/team/user/service-endpoint
// (and the rest of the surface); this file does not build or touch that
// root command.
func newCoreCommands() []*cobra.Command {
	return []*cobra.Command{
		newCoreLoginCmd(),
		newCoreLogoutCmd(),
		newCoreConfigureCmd(),
		newCoreInvokeCmd(),
	}
}
