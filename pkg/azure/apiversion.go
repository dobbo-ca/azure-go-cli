package azure

import (
  "context"
  "fmt"
  "sort"
  "strings"
  "sync"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

var apiVersionCache sync.Map // key: subID|namespace|type|preview, value: string

// ResolveAPIVersion returns the API version to use for the given fully qualified
// resource type (e.g. "Microsoft.Network/virtualNetworks").
// If explicit is non-empty, returns it unchanged. Otherwise queries the provider
// and selects the latest stable version (or includes preview if includePreview).
func ResolveAPIVersion(ctx context.Context, cred azcore.TokenCredential, subID, namespace, resourceType, explicit string, includePreview bool) (string, error) {
  if explicit != "" {
    return explicit, nil
  }
  cacheKey := fmt.Sprintf("%s|%s|%s|%v", subID, namespace, resourceType, includePreview)
  if v, ok := apiVersionCache.Load(cacheKey); ok {
    return v.(string), nil
  }

  client, err := armresources.NewProvidersClient(subID, cred, nil)
  if err != nil {
    return "", fmt.Errorf("failed to create providers client: %w", err)
  }
  resp, err := client.Get(ctx, namespace, nil)
  if err != nil {
    return "", fmt.Errorf("failed to get provider %s: %w", namespace, err)
  }
  for _, rt := range resp.ResourceTypes {
    if rt.ResourceType != nil && strings.EqualFold(*rt.ResourceType, resourceType) {
      versions := make([]string, 0, len(rt.APIVersions))
      for _, v := range rt.APIVersions {
        if v != nil {
          versions = append(versions, *v)
        }
      }
      picked, err := selectLatestAPIVersion(versions, includePreview)
      if err != nil {
        return "", err
      }
      apiVersionCache.Store(cacheKey, picked)
      return picked, nil
    }
  }
  return "", fmt.Errorf("resource type %s not found under provider %s", resourceType, namespace)
}

// selectLatestAPIVersion picks the highest-sorting API version from versions.
// Preview versions are excluded unless includePreview is true. ARM API versions
// sort lexically by date, so reverse string sort gives the newest first.
func selectLatestAPIVersion(versions []string, includePreview bool) (string, error) {
  if len(versions) == 0 {
    return "", fmt.Errorf("no API versions available")
  }
  filtered := make([]string, 0, len(versions))
  for _, v := range versions {
    isPreview := strings.Contains(v, "-preview") || strings.Contains(v, "-beta")
    if isPreview && !includePreview {
      continue
    }
    filtered = append(filtered, v)
  }
  if len(filtered) == 0 {
    return "", fmt.Errorf("no stable API versions available")
  }
  sort.Sort(sort.Reverse(sort.StringSlice(filtered)))
  return filtered[0], nil
}
