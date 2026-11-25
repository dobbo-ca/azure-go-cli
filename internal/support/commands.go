package support

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// tuiAllowed reports whether the TUI may start: both ends must be a terminal and --no-input unset.
func tuiAllowed(noInput, stdinTTY, stdoutTTY bool) bool { return !noInput && stdinTTY && stdoutTTY }

func tuiAllowedFor(cmd *cobra.Command) bool {
	noInput, _ := cmd.Flags().GetBool("no-input")
	return tuiAllowed(noInput, term.IsTerminal(int(os.Stdin.Fd())), term.IsTerminal(int(os.Stdout.Fd())))
}

// runTUI is replaced by the real launcher in the TUI tasks.
func runTUI(cmd *cobra.Command) error { return errors.New("TUI not built yet") }

// NewSupportCommand returns the root `az support` command.
func NewSupportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "support",
		Short:        "Manage Azure support tickets",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if tuiAllowedFor(cmd) {
				return runTUI(cmd)
			}
			return cmd.Help()
		},
	}
	cmd.Flags().String("tenant", "", "Tenant ID or name to start the interactive UI in")
	cmd.Flags().Bool("no-input", false, "Never start the interactive UI")

	tui := &cobra.Command{
		Use:          "tui",
		Short:        "Open the interactive support ticket UI",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !tuiAllowedFor(cmd) {
				return errors.New("az support tui needs an interactive terminal")
			}
			return runTUI(cmd)
		},
	}
	tui.Flags().String("tenant", "", "Tenant ID or name to start the interactive UI in")
	tui.Flags().Bool("no-input", false, "Never start the interactive UI")

	cmd.AddCommand(tui, newServicesCmd(), newScopeGroup("in-subscription", false), newScopeGroup("no-subscription", true))
	return cmd
}

// newScopeGroup builds the "in-subscription" or "no-subscription" command group.
func newScopeGroup(use string, tenantLevel bool) *cobra.Command {
	short := "Manage support tickets in a subscription"
	if tenantLevel {
		short = "Manage support tickets without a subscription (tenant level)"
	}
	cmd := &cobra.Command{Use: use, Short: short, SilenceUsage: true}
	cmd.AddCommand(
		newTicketsCmd(tenantLevel),
		newCommunicationCmd(tenantLevel),
		newChatTranscriptCmd(tenantLevel),
		newFileCmd(tenantLevel),
		newFileWorkspaceCmd(tenantLevel),
	)
	return cmd
}

// group makes a parent command with SilenceUsage set, as cobra only honors it on the executed leaf.
func group(use, short string, subs ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{Use: use, Short: short, SilenceUsage: true}
	for _, s := range subs {
		s.SilenceUsage = true
	}
	cmd.AddCommand(subs...)
	return cmd
}

// truncate applies --max-items after paging; 0 means unset.
func truncate[T any](items []*T, max int) []*T {
	if max > 0 && len(items) > max {
		return items[:max]
	}
	return items
}

// listOrEmpty makes a nil list print as [] rather than null.
func listOrEmpty[T any](items []*T) []*T {
	if items == nil {
		return []*T{}
	}
	return items
}
