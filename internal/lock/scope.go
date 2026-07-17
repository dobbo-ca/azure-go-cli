package lock

import (
  "fmt"
  "strings"

  "github.com/spf13/cobra"
  "github.com/spf13/pflag"
)

type scopeLevel int

const (
  scopeSubscription scopeLevel = iota
  scopeResourceGroup
  scopeResource
)

// scopeKind selects which scope flags a command group registers. The four
// azure-cli lock groups share one implementation and differ only here.
type scopeKind int

const (
  kindGeneric scopeKind = iota // az lock: scope inferred from flags
  kindAccount                  // az account lock: always subscription
  kindGroup                    // az group lock: always resource group
  kindResource                 // az resource lock: always resource
)

// lockScope is the resolved target of a lock operation.
type lockScope struct {
  Level         scopeLevel
  ResourceGroup string
  Namespace     string
  Parent        string
  ResourceType  string
  ResourceName  string
}

// addScopeFlags registers the scope flags appropriate to kind.
//
// --resource and --resource-name are co-equal long aliases in azure-cli, not a
// flag plus a shorthand. A normalize func folds the alias onto one flag so help
// shows a single line.
func addScopeFlags(cmd *cobra.Command, kind scopeKind) {
  switch kind {
  case kindAccount:
    return
  case kindGroup:
    cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
    return
  }

  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("resource", "", "Name or ID of the resource being locked. If an ID is given, other resource arguments should not be given")
  cmd.Flags().String("resource-type", "", "Resource type, qualified (Microsoft.Provider/resC) or bare with --namespace")
  cmd.Flags().String("namespace", "", "Provider namespace, e.g. Microsoft.Provider")
  cmd.Flags().String("parent", "", "Parent path for child resources, e.g. resA/myA/resB/myB")
  cmd.Flags().SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
    if name == "resource-name" {
      name = "resource"
    }
    return pflag.NormalizedName(name)
  })
}

// flagOrEmpty reads a string flag that may not be registered for this kind.
func flagOrEmpty(cmd *cobra.Command, name string) string {
  if cmd.Flags().Lookup(name) == nil {
    return ""
  }
  v, _ := cmd.Flags().GetString(name)
  return v
}

// resolveScope ports azure-cli's internal_validate_lock_parameters.
//
// azure-cli's messages say a flag "is ignored" while actually raising, and one
// carries a missing-space typo. These are rewritten in repo house style.
func resolveScope(cmd *cobra.Command) (lockScope, error) {
  rg := flagOrEmpty(cmd, "resource-group")
  resource := flagOrEmpty(cmd, "resource")
  rtype := flagOrEmpty(cmd, "resource-type")
  ns := flagOrEmpty(cmd, "namespace")
  parent := flagOrEmpty(cmd, "parent")

  if rg == "" {
    if resource != "" {
      if !strings.HasPrefix(resource, "/subscriptions/") {
        return lockScope{}, fmt.Errorf("--resource must be a full resource ID when --resource-group is omitted")
      }
      parts, err := parseResourceScopeID(resource)
      if err != nil {
        return lockScope{}, err
      }
      if rtype != "" {
        return lockScope{}, fmt.Errorf("--resource-type not allowed when --resource is a full resource ID")
      }
      if ns != "" {
        return lockScope{}, fmt.Errorf("--namespace not allowed when --resource is a full resource ID")
      }
      if parent != "" {
        return lockScope{}, fmt.Errorf("--parent not allowed when --resource is a full resource ID")
      }
      parts.Level = scopeResource
      return parts, nil
    }
    if rtype != "" {
      return lockScope{}, fmt.Errorf("--resource-type requires --resource-group")
    }
    if ns != "" {
      return lockScope{}, fmt.Errorf("--namespace requires --resource-group")
    }
    if parent != "" {
      return lockScope{}, fmt.Errorf("--parent requires --resource-group")
    }
    return lockScope{Level: scopeSubscription}, nil
  }

  if resource == "" {
    if rtype != "" {
      return lockScope{}, fmt.Errorf("--resource-type requires --resource")
    }
    if ns != "" {
      return lockScope{}, fmt.Errorf("--namespace requires --resource")
    }
    if parent != "" {
      return lockScope{}, fmt.Errorf("--parent requires --resource")
    }
    return lockScope{Level: scopeResourceGroup, ResourceGroup: rg}, nil
  }

  if rtype == "" {
    return lockScope{}, fmt.Errorf("--resource-type is required when --resource is given")
  }
  segments := strings.Split(rtype, "/")
  switch {
  case len(segments) > 2:
    // azure-cli's split('/', 2) silently leaves a 3-segment type unsplit and
    // malforms the ARM path. Error instead. Spec divergence 4.
    return lockScope{}, fmt.Errorf("--resource-type must be namespace/type; use --parent for child resources")
  case len(segments) == 2:
    if ns != "" {
      return lockScope{}, fmt.Errorf("--namespace given in both --resource-type and --namespace")
    }
    ns, rtype = segments[0], segments[1]
  default:
    if ns == "" {
      return lockScope{}, fmt.Errorf("--resource-type must be namespace/type, or pass --namespace")
    }
  }

  return lockScope{
    Level:         scopeResource,
    ResourceGroup: rg,
    Namespace:     ns,
    Parent:        strings.Trim(parent, "/"),
    ResourceType:  rtype,
    ResourceName:  resource,
  }, nil
}

// parseResourceScopeID back-populates a scope from a full resource ID passed
// to --resource, mirroring azure-cli's use of parse_resource_id there.
func parseResourceScopeID(id string) (lockScope, error) {
  trimmed := strings.Trim(id, "/")
  parts := strings.Split(trimmed, "/")
  if len(parts) < 8 || !strings.EqualFold(parts[2], "resourcegroups") || !strings.EqualFold(parts[4], "providers") {
    return lockScope{}, fmt.Errorf("--resource is not a valid resource ID: %s", id)
  }
  rest := parts[6:]
  if len(rest)%2 != 0 {
    return lockScope{}, fmt.Errorf("--resource is not a valid resource ID: %s", id)
  }
  s := lockScope{ResourceGroup: parts[3], Namespace: parts[5]}
  s.ResourceType = rest[len(rest)-2]
  s.ResourceName = rest[len(rest)-1]
  if len(rest) > 2 {
    s.Parent = strings.Join(rest[:len(rest)-2], "/")
  }
  return s, nil
}
