package config

import (
  "encoding/json"
  "fmt"
  "os"
  "path/filepath"

  "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
  ConfigDir       = ".azure"
  ConfigFile      = "azureProfile.json"
  TokenCacheFile  = "accessTokens.json"
  AuthRecordFile  = "authRecord.json"
)

type Profile struct {
  Subscriptions       []Subscription                  `json:"subscriptions"`
  AuthenticationRecord *azidentity.AuthenticationRecord `json:"authenticationRecord,omitempty"`
}

type Subscription struct {
  ID              string `json:"id"`
  Name            string `json:"name"`
  State           string `json:"state"`
  User            User   `json:"user"`
  IsDefault       bool   `json:"isDefault"`
  TenantID        string `json:"tenantId"`
  EnvironmentName string `json:"environmentName"`
  HomeTenantID    string `json:"homeTenantId"`
}

type User struct {
  Name string `json:"name"`
  Type string `json:"type"`
}

func GetConfigPath() (string, error) {
  home, err := os.UserHomeDir()
  if err != nil {
    return "", fmt.Errorf("failed to get home directory: %w", err)
  }

  configPath := filepath.Join(home, ConfigDir, ConfigFile)
  return configPath, nil
}

func GetTokenCachePath() (string, error) {
  home, err := os.UserHomeDir()
  if err != nil {
    return "", fmt.Errorf("failed to get home directory: %w", err)
  }

  cachePath := filepath.Join(home, ConfigDir, TokenCacheFile)
  return cachePath, nil
}

func Save(profile *Profile) error {
  configPath, err := GetConfigPath()
  if err != nil {
    return err
  }

  configDirPath := filepath.Dir(configPath)
  if err := os.MkdirAll(configDirPath, 0700); err != nil {
    return fmt.Errorf("failed to create config directory: %w", err)
  }

  data, err := json.MarshalIndent(profile, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to marshal profile: %w", err)
  }

  if err := os.WriteFile(configPath, data, 0600); err != nil {
    return fmt.Errorf("failed to write profile: %w", err)
  }

  return nil
}

func Load() (*Profile, error) {
  configPath, err := GetConfigPath()
  if err != nil {
    return nil, err
  }

  data, err := os.ReadFile(configPath)
  if err != nil {
    if os.IsNotExist(err) {
      return nil, fmt.Errorf("not logged in. Please run 'az login'")
    }
    return nil, fmt.Errorf("failed to read profile: %w", err)
  }

  var profile Profile
  if err := json.Unmarshal(data, &profile); err != nil {
    return nil, fmt.Errorf("failed to parse profile: %w", err)
  }

  return &profile, nil
}

func GetDefaultSubscription() (string, error) {
  profile, err := Load()
  if err != nil {
    return "", err
  }

  for i := range profile.Subscriptions {
    if profile.Subscriptions[i].IsDefault {
      return profile.Subscriptions[i].ID, nil
    }
  }

  if len(profile.Subscriptions) > 0 {
    return profile.Subscriptions[0].ID, nil
  }

  return "", fmt.Errorf("no subscription found")
}

func Delete() error {
  configPath, err := GetConfigPath()
  if err != nil {
    return err
  }

  // Remove profile
  if _, err := os.Stat(configPath); err == nil {
    if err := os.Remove(configPath); err != nil {
      return fmt.Errorf("failed to remove profile: %w", err)
    }
  }

  // Also clear MSAL cache files to ensure complete logout
  // This prevents old tokens from persisting across different accounts
  home, err := os.UserHomeDir()
  if err == nil {
    azureDir := filepath.Join(home, ConfigDir)

    // Remove MSAL token cache
    msalTokenCache := filepath.Join(azureDir, "msal_token_cache.json")
    if _, err := os.Stat(msalTokenCache); err == nil {
      _ = os.Remove(msalTokenCache) // Ignore errors, best effort
    }

    // Remove MSAL HTTP cache (created by Azure SDK)
    msalHttpCache := filepath.Join(azureDir, "msal_http_cache.bin")
    if _, err := os.Stat(msalHttpCache); err == nil {
      _ = os.Remove(msalHttpCache) // Ignore errors, best effort
    }
  }

  return nil
}
