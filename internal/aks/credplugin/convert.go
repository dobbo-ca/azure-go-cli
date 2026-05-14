package credplugin

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ConvertOptions controls how Convert emits the exec entry.
type ConvertOptions struct {
	// AbsolutePath, when true, uses os.Executable() result as the exec
	// command field instead of the bare string "az".
	AbsolutePath bool
}

// Convert rewrites a kubeconfig in-memory, replacing legacy `auth-provider: azure`
// blocks and existing `kubelogin` exec entries with exec entries pointing at
// this binary. Returns the new bytes, a flag indicating whether anything
// changed, and any parse/marshal error.
func Convert(kubeConfig []byte, opts ConvertOptions) ([]byte, bool, error) {
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(kubeConfig, &cfg); err != nil {
		return nil, false, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	command, err := commandField(opts.AbsolutePath)
	if err != nil {
		return nil, false, err
	}

	users, _ := cfg["users"].([]interface{})
	changed := false
	for _, item := range users {
		userEntry, _ := item.(map[string]interface{})
		if userEntry == nil {
			continue
		}
		userMap, _ := userEntry["user"].(map[string]interface{})
		if userMap == nil {
			continue
		}
		if rewriteLegacyAuthProvider(userMap, command) {
			changed = true
		}
	}

	if !changed {
		return kubeConfig, false, nil
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}
	return out, true, nil
}

// commandField returns the string to use for the exec.command field.
func commandField(absolute bool) (string, error) {
	if !absolute {
		return "az", nil
	}
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}
	return p, nil
}

// rewriteLegacyAuthProvider replaces a `user.auth-provider: azure` block with a
// matching `user.exec` block. Returns true if the user was rewritten.
func rewriteLegacyAuthProvider(userMap map[string]interface{}, command string) bool {
	ap, _ := userMap["auth-provider"].(map[string]interface{})
	if ap == nil {
		return false
	}
	if name, _ := ap["name"].(string); name != "azure" {
		return false
	}
	cfg, _ := ap["config"].(map[string]interface{})

	serverID := AKSServerIDDefault
	if v, _ := cfg["apiserver-id"].(string); v != "" {
		serverID = v
	}
	tenantID, _ := cfg["tenant-id"].(string)
	clientID, _ := cfg["client-id"].(string)

	delete(userMap, "auth-provider")
	userMap["exec"] = buildExecEntry(command, serverID, tenantID, clientID)
	return true
}

// buildExecEntry constructs the standard exec entry pointing at this binary.
// env is left nil; the bastion temp-kubeconfig path populates env directly
// (see internal/aks/kubeconfig.go), not via Convert.
func buildExecEntry(command, serverID, tenantID, clientID string) map[string]interface{} {
	args := []interface{}{"aks", "get-token", "--server-id", serverID}
	if tenantID != "" {
		args = append(args, "--tenant-id", tenantID)
	}
	if clientID != "" {
		args = append(args, "--client-id", clientID)
	}
	return map[string]interface{}{
		"apiVersion":         APIVersionV1Beta1,
		"command":            command,
		"args":               args,
		"interactiveMode":    "IfAvailable",
		"provideClusterInfo": false,
	}
}
