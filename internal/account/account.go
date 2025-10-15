package account

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List() error {
  // Fetch fresh subscription list from Azure API across all tenants using cached credentials
  ctx := context.Background()
  tenantInfos, err := azure.DiscoverAllSubscriptionsFromCache(ctx)
  if err != nil {
    return fmt.Errorf("failed to discover subscriptions: %w", err)
  }

  allSubscriptions := azure.GetAllSubscriptions(tenantInfos)

  // Load profile to see which subscription is marked as default
  profile, err := config.Load()
  var defaultSubID string
  if err == nil {
    for _, sub := range profile.Subscriptions {
      if sub.IsDefault {
        defaultSubID = sub.ID
        break
      }
    }
  }

  // Mark default subscription
  for i := range allSubscriptions {
    allSubscriptions[i].IsDefault = (allSubscriptions[i].ID == defaultSubID)
  }

  data, err := json.MarshalIndent(allSubscriptions, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format subscriptions: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

func Show() error {
  profile, err := config.Load()
  if err != nil {
    return err
  }

  var defaultSub *config.Subscription
  for i := range profile.Subscriptions {
    if profile.Subscriptions[i].IsDefault {
      defaultSub = &profile.Subscriptions[i]
      break
    }
  }

  if defaultSub == nil && len(profile.Subscriptions) > 0 {
    defaultSub = &profile.Subscriptions[0]
  }

  if defaultSub == nil {
    return fmt.Errorf("no subscription found")
  }

  data, err := json.MarshalIndent(defaultSub, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format subscription: %w", err)
  }

  fmt.Println(string(data))
  return nil
}

func Set(subscriptionID string) error {
  if subscriptionID == "" {
    return fmt.Errorf("subscription ID or name required")
  }

  // Fetch fresh subscription list from Azure API across all tenants using cached credentials
  ctx := context.Background()
  tenantInfos, err := azure.DiscoverAllSubscriptionsFromCache(ctx)
  if err != nil {
    return fmt.Errorf("failed to discover subscriptions: %w", err)
  }

  allSubscriptions := azure.GetAllSubscriptions(tenantInfos)

  // Find the matching subscription
  var foundSub *config.Subscription
  for i := range allSubscriptions {
    if allSubscriptions[i].ID == subscriptionID || allSubscriptions[i].Name == subscriptionID {
      allSubscriptions[i].IsDefault = true
      foundSub = &allSubscriptions[i]
    } else {
      allSubscriptions[i].IsDefault = false
    }
  }

  if foundSub == nil {
    return fmt.Errorf("subscription '%s' not found", subscriptionID)
  }

  // Load existing profile to preserve auth record
  profile, err := config.Load()
  if err != nil {
    return err
  }

  // Update subscriptions list with fresh data and new default
  profile.Subscriptions = allSubscriptions

  if err := config.Save(profile); err != nil {
    return fmt.Errorf("failed to save profile: %w", err)
  }

  fmt.Printf("Subscription set to: %s (%s)\n", foundSub.Name, foundSub.ID)
  return nil
}

func Clear() error {
  return config.Delete()
}
