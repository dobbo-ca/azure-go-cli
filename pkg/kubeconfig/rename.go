package kubeconfig

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// RenameByRegex applies pattern.ReplaceAllString to the kubeconfig's cluster
// name (clusters[0].name) to derive a new name, then replaces every substring
// occurrence of the old name with the new name across identifier fields:
// current-context, clusters[].name, contexts[].name, contexts[].context.cluster,
// contexts[].context.user, users[].name.
//
// If the input has no cluster name or the regex does not transform it, the
// kubeconfig is returned unchanged (semantically; YAML formatting may differ
// only if the input itself was non-canonical).
func RenameByRegex(kubeConfig []byte, pattern *regexp.Regexp, replacement string) ([]byte, error) {
	if pattern == nil {
		return kubeConfig, nil
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(kubeConfig, &cfg); err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}

	clusters, _ := cfg["clusters"].([]interface{})
	if len(clusters) == 0 {
		return kubeConfig, nil
	}
	firstCluster, _ := clusters[0].(map[string]interface{})
	oldName, _ := firstCluster["name"].(string)
	if oldName == "" {
		return kubeConfig, nil
	}

	newName := pattern.ReplaceAllString(oldName, replacement)
	if newName == oldName {
		return kubeConfig, nil
	}

	replace := func(s string) string {
		return strings.ReplaceAll(s, oldName, newName)
	}

	if cc, ok := cfg["current-context"].(string); ok {
		cfg["current-context"] = replace(cc)
	}

	for _, key := range []string{"clusters", "contexts", "users"} {
		list, _ := cfg[key].([]interface{})
		for _, item := range list {
			m, _ := item.(map[string]interface{})
			if m == nil {
				continue
			}
			if n, ok := m["name"].(string); ok {
				m["name"] = replace(n)
			}
			if ctx, ok := m["context"].(map[string]interface{}); ok {
				if c, ok := ctx["cluster"].(string); ok {
					ctx["cluster"] = replace(c)
				}
				if u, ok := ctx["user"].(string); ok {
					ctx["user"] = replace(u)
				}
			}
		}
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal kubeconfig: %w", err)
	}
	return out, nil
}
