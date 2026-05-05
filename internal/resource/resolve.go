package resource

import (
  "fmt"
  "strings"
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
