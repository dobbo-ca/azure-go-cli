package config

import (
  "fmt"
  "os"
  "path/filepath"
  "strings"
)

const (
  ConfigFileName = "config"
)

// Settings represents the CLI configuration
type Settings struct {
  Core CoreSettings `ini:"core"`
}

type CoreSettings struct {
  // credential_storage can be "file" or "keychain"
  CredentialStorage string `ini:"credential_storage"`
  // default_subscription can be set to a subscription ID
  DefaultSubscription string `ini:"default_subscription"`
}

// LoadSettings loads configuration from ~/.azure/config
func LoadSettings() (*Settings, error) {
  configPath, err := getConfigFilePath()
  if err != nil {
    return nil, err
  }

  // Return defaults if config file doesn't exist
  if _, err := os.Stat(configPath); os.IsNotExist(err) {
    return &Settings{
      Core: CoreSettings{
        CredentialStorage: "file", // Default to file-based storage
      },
    }, nil
  }

  data, err := os.ReadFile(configPath)
  if err != nil {
    return nil, fmt.Errorf("failed to read config file: %w", err)
  }

  settings := &Settings{
    Core: CoreSettings{
      CredentialStorage: "file", // Default to file
    },
  }

  if err := parseINI(string(data), settings); err != nil {
    return nil, fmt.Errorf("failed to parse config: %w", err)
  }

  return settings, nil
}

// SaveSettings saves configuration to ~/.azure/config
func SaveSettings(settings *Settings) error {
  configPath, err := getConfigFilePath()
  if err != nil {
    return err
  }

  // Ensure directory exists
  configDir := filepath.Dir(configPath)
  if err := os.MkdirAll(configDir, 0700); err != nil {
    return fmt.Errorf("failed to create config directory: %w", err)
  }

  content := formatINI(settings)
  if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
    return fmt.Errorf("failed to write config file: %w", err)
  }

  return nil
}

func getConfigFilePath() (string, error) {
  home, err := os.UserHomeDir()
  if err != nil {
    return "", fmt.Errorf("failed to get home directory: %w", err)
  }

  return filepath.Join(home, ConfigDir, ConfigFileName), nil
}

// Simple INI parser
func parseINI(content string, settings *Settings) error {
  var currentSection string
  lines := strings.Split(content, "\n")

  for _, line := range lines {
    line = strings.TrimSpace(line)

    // Skip empty lines and comments
    if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
      continue
    }

    // Section header
    if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
      currentSection = strings.Trim(line, "[]")
      continue
    }

    // Key-value pair
    parts := strings.SplitN(line, "=", 2)
    if len(parts) != 2 {
      continue
    }

    key := strings.TrimSpace(parts[0])
    value := strings.TrimSpace(parts[1])

    // Parse based on section
    if currentSection == "core" {
      switch key {
      case "credential_storage":
        settings.Core.CredentialStorage = value
      case "default_subscription":
        settings.Core.DefaultSubscription = value
      }
    }
  }

  return nil
}

// Format settings as INI
func formatINI(settings *Settings) string {
  var sb strings.Builder

  sb.WriteString("[core]\n")
  sb.WriteString(fmt.Sprintf("credential_storage = %s\n", settings.Core.CredentialStorage))
  if settings.Core.DefaultSubscription != "" {
    sb.WriteString(fmt.Sprintf("default_subscription = %s\n", settings.Core.DefaultSubscription))
  }

  return sb.String()
}
