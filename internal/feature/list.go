package feature

import (
  "context"
  "encoding/json"
  "fmt"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armfeatures"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

type FeatureInfo struct {
  Name     string `json:"name"`
  Provider string `json:"provider"`
  State    string `json:"state"`
  ID       string `json:"id"`
}

func List(ctx context.Context, provider string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armfeatures.NewClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create features client: %w", err)
  }

  var features []FeatureInfo

  if provider != "" {
    // List features for specific provider
    pager := client.NewListPager(provider, nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to get feature page: %w", err)
      }

      for _, feature := range page.Value {
        featureInfo := formatFeature(feature)
        features = append(features, featureInfo)
      }
    }
  } else {
    // List all features across all providers
    pager := client.NewListAllPager(nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to get feature page: %w", err)
      }

      for _, feature := range page.Value {
        featureInfo := formatFeature(feature)
        features = append(features, featureInfo)
      }
    }
  }

  if len(features) == 0 {
    if provider != "" {
      fmt.Printf("No features found for provider '%s'\n", provider)
    } else {
      fmt.Println("No features found")
    }
    return nil
  }

  data, err := json.MarshalIndent(features, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format features: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

func formatFeature(feature *armfeatures.FeatureResult) FeatureInfo {
  info := FeatureInfo{
    ID: azure.GetStringValue(feature.ID),
  }

  // Extract feature name and provider from the full name
  // Format is typically: "{provider}/{featureName}"
  fullName := azure.GetStringValue(feature.Name)
  if fullName != "" {
    parts := strings.Split(fullName, "/")
    if len(parts) == 2 {
      info.Provider = parts[0]
      info.Name = parts[1]
    } else {
      info.Name = fullName
    }
  }

  // Get state from properties
  if feature.Properties != nil && feature.Properties.State != nil {
    info.State = *feature.Properties.State
  }

  return info
}
