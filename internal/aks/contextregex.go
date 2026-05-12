package aks

import (
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
)

// addContextRegexFlags registers --context-regex and --context-replacement on cmd.
func addContextRegexFlags(cmd *cobra.Command) {
	cmd.Flags().String("context-regex", "",
		"Regex matched against the cluster name; the replacement is propagated to every kubeconfig identifier. Requires --context-replacement.")
	cmd.Flags().String("context-replacement", "",
		"Replacement string for --context-regex (supports $1, $2 capture group references).")
}

// parseContextRegexFlags compiles --context-regex / --context-replacement and
// enforces the pair-required and mutual-exclusion constraints. Returns a nil
// pattern when neither flag is set. `literalContext` is the value of the
// existing --context flag (pass "" if the command does not register it).
func parseContextRegexFlags(cmd *cobra.Command, literalContext string) (*regexp.Regexp, string, error) {
	pattern, _ := cmd.Flags().GetString("context-regex")
	replacement, _ := cmd.Flags().GetString("context-replacement")

	regexSet := cmd.Flags().Changed("context-regex")
	replSet := cmd.Flags().Changed("context-replacement")

	if regexSet != replSet {
		return nil, "", fmt.Errorf("--context-regex and --context-replacement must be supplied together")
	}
	if !regexSet {
		return nil, "", nil
	}
	if literalContext != "" {
		return nil, "", fmt.Errorf("--context is mutually exclusive with --context-regex")
	}

	if pattern == "" {
		return nil, "", fmt.Errorf("--context-regex cannot be empty")
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, "", fmt.Errorf("invalid --context-regex: %w", err)
	}
	return compiled, replacement, nil
}
