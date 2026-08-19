package ado

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configFile = "config"

// parseINI parses a minimal INI document (comments prefixed "#" or ";",
// "[section]" headers, "key = value" pairs) into section -> lowercased key
// -> value. It is the one INI reader for this package — both the devops
// config file and the PAT store use it.
func parseINI(data string) map[string]map[string]string {
	sections := map[string]map[string]string{}

	var section string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			if _, ok := sections[section]; !ok {
				sections[section] = map[string]string{}
			}
			continue
		}
		if section == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		sections[section][key] = strings.TrimSpace(parts[1])
	}

	return sections
}

// ConfigDefaults returns the [defaults] organization/project values from
// <configDir>/config. CONFIG_VALID_DEFAULT_KEYS_LIST (configure.py:23) is
// exactly ['organization','project'] — there is no default for repository.
func ConfigDefaults() (org, project string, err error) {
	data, err := os.ReadFile(filepath.Join(configDir(), configFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("failed to read devops config: %w", err)
	}

	defaults := parseINI(string(data))["defaults"]
	return defaults["organization"], defaults["project"], nil
}

// SetConfigDefaults writes the [defaults] section of <configDir>/config. An
// empty org or project omits that key rather than clearing it — callers that
// want to unset a key handle that themselves.
func SetConfigDefaults(org, project string) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var b strings.Builder
	b.WriteString("[defaults]\n")
	if org != "" {
		fmt.Fprintf(&b, "organization = %s\n", org)
	}
	if project != "" {
		fmt.Fprintf(&b, "project = %s\n", project)
	}

	if err := os.WriteFile(filepath.Join(dir, configFile), []byte(b.String()), 0600); err != nil {
		return fmt.Errorf("failed to write devops config: %w", err)
	}
	return nil
}
