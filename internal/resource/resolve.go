package resource

import (
  "fmt"
  "strings"

  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

// ParseResourceID splits an ARM resource ID into its components.
// Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/{ns}/{type}/{name}[/{type}/{name}...]
func ParseResourceID(id string) (sub, group, namespace string, types, names []string, err error) {
  if id == "" {
    return "", "", "", nil, nil, fmt.Errorf("resource ID is empty")
  }
  parts := strings.Split(strings.TrimPrefix(id, "/"), "/")
  if len(parts) < 8 || parts[0] != "subscriptions" || parts[2] != "resourceGroups" || parts[4] != "providers" {
    return "", "", "", nil, nil, fmt.Errorf("invalid resource ID: %s", id)
  }
  sub = parts[1]
  group = parts[3]
  namespace = parts[5]
  remainder := parts[6:]
  if len(remainder)%2 != 0 {
    return "", "", "", nil, nil, fmt.Errorf("invalid resource ID type/name pairing: %s", id)
  }
  for i := 0; i < len(remainder); i += 2 {
    types = append(types, remainder[i])
    names = append(names, remainder[i+1])
  }
  return sub, group, namespace, types, names, nil
}

// BuildResourceID assembles an ARM resource ID from name-mode flag inputs.
// resourceType may be qualified ("Microsoft.X/y") or unqualified ("y") if namespace is given.
// parent is an optional "type/name[/type/name...]" prefix for child resources.
func BuildResourceID(sub, group, namespace, resourceType, parent, name string) (string, error) {
  if sub == "" || group == "" || resourceType == "" || name == "" {
    return "", fmt.Errorf("subscription, resource group, resource type, and name are all required")
  }

  ns := namespace
  rt := resourceType
  if strings.Contains(resourceType, "/") {
    // qualified: split first segment as namespace
    idx := strings.Index(resourceType, "/")
    ns = resourceType[:idx]
    rt = resourceType[idx+1:]
  }
  if ns == "" {
    return "", fmt.Errorf("namespace required when --resource-type is unqualified")
  }

  parentPart := ""
  if parent != "" {
    parentPart = "/" + strings.Trim(parent, "/")
  }

  return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/%s%s/%s/%s",
    sub, group, ns, parentPart, rt, name), nil
}

// AddSelectorFlags registers the resource-selector flag set on cmd.
// Used by every subcommand that operates on a specific resource.
func AddSelectorFlags(cmd *cobra.Command) {
  cmd.Flags().StringSlice("ids", nil, "One or more resource IDs (space- or comma-separated). If supplied, no other resource arguments should be specified.")
  cmd.Flags().StringP("name", "n", "", "Resource name. Required when --ids is not given.")
  cmd.Flags().StringP("resource-group", "g", "", "Resource group. Required when --ids is not given.")
  cmd.Flags().String("resource-type", "", "Resource type, qualified (Microsoft.Foo/bar) or unqualified with --namespace.")
  cmd.Flags().String("namespace", "", "Provider namespace, e.g. Microsoft.Network.")
  cmd.Flags().String("parent", "", "Parent path for child resources (e.g. virtualNetworks/myvnet).")
}

// ResolveSelector returns the resource IDs implied by the flags on cmd.
// Returns multiple IDs only when --ids was used.
func ResolveSelector(cmd *cobra.Command) ([]string, error) {
  ids, _ := cmd.Flags().GetStringSlice("ids")
  name, _ := cmd.Flags().GetString("name")
  group, _ := cmd.Flags().GetString("resource-group")
  rtype, _ := cmd.Flags().GetString("resource-type")
  namespace, _ := cmd.Flags().GetString("namespace")
  parent, _ := cmd.Flags().GetString("parent")

  hasIDs := len(ids) > 0
  hasName := name != "" || group != "" || rtype != ""

  if hasIDs && hasName {
    return nil, fmt.Errorf("cannot mix --ids with -g/--resource-type/-n")
  }
  if !hasIDs && !hasName {
    return nil, fmt.Errorf("please specify either --ids or both -g and resource info")
  }

  if hasIDs {
    return ids, nil
  }

  if name == "" || group == "" || rtype == "" {
    return nil, fmt.Errorf("--resource-group, --resource-type, and --name are all required when --ids is not given")
  }

  sub, _ := cmd.Flags().GetString("subscription")
  if sub == "" {
    var err error
    sub, err = config.GetDefaultSubscription()
    if err != nil {
      return nil, err
    }
  }

  id, err := BuildResourceID(sub, group, namespace, rtype, parent, name)
  if err != nil {
    return nil, err
  }
  return []string{id}, nil
}
