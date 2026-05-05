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
