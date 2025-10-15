package auth

import (
  "context"
  "fmt"
  "strings"

  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Login(ctx context.Context) error {
  fmt.Println("A web browser has been opened at https://login.microsoftonline.com/organizations/oauth2/v2.0/authorize. Please continue the login in the web browser. If no web browser is available or if the web browser fails to open, use device code flow with `az login --use-device-code`.")
  fmt.Println()

  // Use interactive browser credential (matches official Azure CLI behavior)
  cred, err := azure.GetInteractiveBrowserCredentialWithCache()
  if err != nil {
    return fmt.Errorf("failed to create credential: %w", err)
  }

  // Discover subscriptions across all tenants
  // This will:
  // 1. Trigger interactive browser authentication (ONE TIME)
  // 2. List all tenants (home + guest)
  // 3. For each tenant, get subscriptions silently using cached tokens
  fmt.Println("Retrieving tenants and subscriptions for the selection...")
  tenantInfos, authRecord, err := azure.DiscoverAllSubscriptionsWithAuth(ctx, cred)
  if err != nil {
    return fmt.Errorf("failed to discover subscriptions: %w", err)
  }

  allSubscriptions := azure.GetAllSubscriptions(tenantInfos)
  if len(allSubscriptions) == 0 {
    return fmt.Errorf("no subscriptions found")
  }

  // Display subscriptions and let user select
  fmt.Println("\n[Tenant and subscription selection]\n")
  selectedSub, err := promptForSubscription(tenantInfos)
  if err != nil {
    return fmt.Errorf("failed to select subscription: %w", err)
  }

  // Mark the selected subscription as default
  for i := range allSubscriptions {
    allSubscriptions[i].IsDefault = (allSubscriptions[i].ID == selectedSub.ID)
  }

  // Save profile with authentication record and subscriptions
  profile := config.Profile{
    Subscriptions:        allSubscriptions,
    AuthenticationRecord: &authRecord,
  }

  if err := config.Save(&profile); err != nil {
    return fmt.Errorf("failed to save profile: %w", err)
  }

  fmt.Printf("\nTenant: %s\n", selectedSub.TenantID)
  fmt.Printf("Subscription: %s (%s)\n", selectedSub.Name, selectedSub.ID)
  fmt.Println("\nYou have successfully logged in.")

  return nil
}

func promptForSubscription(tenantInfos []azure.TenantInfo) (*config.Subscription, error) {
  // Build a flat list of subscriptions with indices
  type subscriptionChoice struct {
    Index        int
    Subscription config.Subscription
    TenantName   string
  }

  var choices []subscriptionChoice
  idx := 1

  for _, tenant := range tenantInfos {
    tenantDisplay := tenant.DisplayName
    if tenantDisplay == "" {
      tenantDisplay = tenant.TenantID
    }

    for _, sub := range tenant.Subscriptions {
      choices = append(choices, subscriptionChoice{
        Index:        idx,
        Subscription: sub,
        TenantName:   tenantDisplay,
      })
      idx++
    }
  }

  // Display table
  fmt.Printf("%-6s %-30s %-38s %-20s\n", "No", "Subscription name", "Subscription ID", "Tenant")
  fmt.Println(strings.Repeat("-", 100))

  for i, choice := range choices {
    marker := " "
    if i == 0 {
      marker = "*"
    }
    fmt.Printf("[%d] %s  %-30s %-38s %-20s\n",
      choice.Index,
      marker,
      truncate(choice.Subscription.Name, 30),
      choice.Subscription.ID,
      truncate(choice.TenantName, 20))
  }

  fmt.Printf("\nThe default is marked with an *; the default tenant is '%s' and subscription is '%s' (%s).\n\n",
    choices[0].TenantName,
    choices[0].Subscription.Name,
    choices[0].Subscription.ID)

  fmt.Print("Select a subscription and tenant (Type a number or Enter for no changes): ")

  var input string
  fmt.Scanln(&input)
  input = strings.TrimSpace(input)

  // If empty, use default (first subscription)
  if input == "" {
    return &choices[0].Subscription, nil
  }

  // Parse the number
  var selectedNum int
  if _, err := fmt.Sscanf(input, "%d", &selectedNum); err != nil {
    return nil, fmt.Errorf("invalid selection: %s", input)
  }

  // Find the matching choice
  for _, choice := range choices {
    if choice.Index == selectedNum {
      return &choice.Subscription, nil
    }
  }

  return nil, fmt.Errorf("invalid selection: %d", selectedNum)
}

func truncate(s string, maxLen int) string {
  if len(s) <= maxLen {
    return s
  }
  return s[:maxLen-3] + "..."
}
