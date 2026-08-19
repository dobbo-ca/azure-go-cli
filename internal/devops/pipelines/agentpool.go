// This file and the other agentpool_*.go files implement the `az pipelines
// pool`, `az pipelines agent` and `az pipelines queue` command groups
// (agent_pool_queue.py). Other pipelines subgroups (release,
// variable-group, variable, folder, ...) are implemented in sibling files
// by other contributors.
package pipelines

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newAgentPoolCommands wires `az pipelines pool`, `az pipelines agent` and
// `az pipelines queue`. These are three separate sibling groups, not one
// shared parent — Python registers them as three distinct command_group
// blocks (commands.py:157-167) with no common parent beyond `pipelines`
// itself.
func newAgentPoolCommands() []*cobra.Command {
	return []*cobra.Command{
		agentpoolNewPoolCmd(),
		agentpoolNewAgentCmd(),
		agentpoolNewQueueCmd(),
	}
}

func agentpoolNewPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage agent pools",
		Long:  "Manage Azure DevOps agent pools",
	}
	cmd.AddCommand(agentpoolNewPoolListCmd())
	cmd.AddCommand(agentpoolNewPoolShowCmd())
	return cmd
}

func agentpoolNewAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agents",
		Long:  "Manage Azure DevOps agents",
	}
	cmd.AddCommand(agentpoolNewAgentListCmd())
	cmd.AddCommand(agentpoolNewAgentShowCmd())
	return cmd
}

func agentpoolNewQueueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage agent queues",
		Long:  "Manage Azure DevOps agent queues",
	}
	cmd.AddCommand(agentpoolNewQueueListCmd())
	cmd.AddCommand(agentpoolNewQueueShowCmd())
	return cmd
}

// agentpoolActionChoices is _ACTION_FILTER_TYPES (arguments.py:23).
var agentpoolActionChoices = []string{"use", "manage", "none"}

// agentpoolPoolTypeChoices is _AGENT_POOL_TYPES (arguments.py:21).
var agentpoolPoolTypeChoices = []string{"automation", "deployment"}

// agentpoolValidateChoice checks value against allowed when value is
// non-empty and returns the canonical (allowed-list) casing, matching
// knack's enum_choice_list: it builds a CaseInsensitiveList validator and
// normalises the parsed value to the canonical entry (arguments.py:92-93,
// 103,108,110), so `--action Use` is accepted and sent on the wire as
// "use", not "Use".
func agentpoolValidateChoice(value, flag string, allowed []string) (string, error) {
	if value == "" {
		return "", nil
	}
	for _, a := range allowed {
		if strings.EqualFold(value, a) {
			return a, nil
		}
	}
	return "", fmt.Errorf("--%s must be one of %s", flag, strings.Join(allowed, ", "))
}

// agentpoolAddThreeStateFlag registers name as a tri-state bool flag (unset,
// "true", or "false"), matching Python's get_three_state_flag() the same
// way ado.AddOrgFlags does it for --detect: a plain string flag with
// NoOptDefVal so the bare flag means true, and an explicit --name=false
// still works.
func agentpoolAddThreeStateFlag(cmd *cobra.Command, name, usage string) {
	cmd.Flags().String(name, "", usage)
	cmd.Flags().Lookup(name).NoOptDefVal = "true"
}

// agentpoolThreeState reads a flag registered by agentpoolAddThreeStateFlag:
// nil means unset (omit the query parameter entirely, matching Python's
// include_capabilities=None default), non-nil means the explicit bool.
func agentpoolThreeState(cmd *cobra.Command, name string) (*bool, error) {
	v, _ := cmd.Flags().GetString(name)
	switch {
	case v == "":
		return nil, nil
	case strings.EqualFold(v, "true"):
		b := true
		return &b, nil
	case strings.EqualFold(v, "false"):
		b := false
		return &b, nil
	default:
		return nil, fmt.Errorf("invalid value %q for --%s; must be true or false", v, name)
	}
}

// agentpoolRequiredIntFlag reads primary, falling back to alias, and errors
// if neither was set. Used for the --pool-id/--id pair on `pipelines pool
// show`.
// ponytail: 0 doubles as "unset" here (no real pool ID is ever 0), same
// simplification the alias pair otherwise needs a third bool for.
func agentpoolRequiredIntFlag(cmd *cobra.Command, primary, alias string) (int, error) {
	v, _ := cmd.Flags().GetInt(primary)
	if v == 0 {
		v, _ = cmd.Flags().GetInt(alias)
	}
	if v == 0 {
		return 0, fmt.Errorf("--%s is required", primary)
	}
	return v, nil
}

// agentpoolRequiredStringFlag reads primary, falling back to alias, and
// errors if neither was set. Used for the --agent-id/--id and
// --queue-id/--id pairs.
func agentpoolRequiredStringFlag(cmd *cobra.Command, primary, alias string) (string, error) {
	v, _ := cmd.Flags().GetString(primary)
	if v == "" {
		v, _ = cmd.Flags().GetString(alias)
	}
	if v == "" {
		return "", fmt.Errorf("--%s is required", primary)
	}
	return v, nil
}
