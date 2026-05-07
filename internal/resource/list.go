package resource

import (
  "context"
  "fmt"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List resources",
    Long:  "List resources, optionally filtered by group, type, name, location, or tag.",
    RunE:  runList,
  }
  cmd.Flags().StringP("name", "n", "", "Filter by resource name")
  cmd.Flags().StringP("resource-group", "g", "", "Limit to a single resource group")
  cmd.Flags().String("resource-type", "", "Filter by resource type (e.g. Microsoft.Network/virtualNetworks)")
  cmd.Flags().String("namespace", "", "Provider namespace (combined with --resource-type if unqualified)")
  cmd.Flags().StringP("location", "l", "", "Filter by location")
  cmd.Flags().String("tag", "", "Filter by tag (key or key=value)")
  return cmd
}

func runList(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  client, _, _, err := newGenericClient(cmd)
  if err != nil {
    return err
  }

  filter := buildListFilter(cmd)
  group, _ := cmd.Flags().GetString("resource-group")

  var results []map[string]interface{}
  if group != "" {
    opts := &armresources.ClientListByResourceGroupOptions{}
    if filter != "" {
      opts.Filter = &filter
    }
    pager := client.NewListByResourceGroupPager(group, opts)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("list failed: %w", err)
      }
      for _, r := range page.Value {
        results = append(results, genericResourceToMap(r))
      }
    }
  } else {
    opts := &armresources.ClientListOptions{}
    if filter != "" {
      opts.Filter = &filter
    }
    pager := client.NewListPager(opts)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("list failed: %w", err)
      }
      for _, r := range page.Value {
        results = append(results, genericResourceToMap(r))
      }
    }
  }

  if results == nil {
    results = []map[string]interface{}{}
  }
  return output.PrintJSON(cmd, results)
}

func buildListFilter(cmd *cobra.Command) string {
  name, _ := cmd.Flags().GetString("name")
  rtype, _ := cmd.Flags().GetString("resource-type")
  namespace, _ := cmd.Flags().GetString("namespace")
  location, _ := cmd.Flags().GetString("location")
  tag, _ := cmd.Flags().GetString("tag")

  // Combine namespace+type if --resource-type is unqualified.
  if rtype != "" && !strings.Contains(rtype, "/") && namespace != "" {
    rtype = namespace + "/" + rtype
  }

  parts := []string{}
  if name != "" {
    parts = append(parts, fmt.Sprintf("name eq '%s'", name))
  }
  if rtype != "" {
    parts = append(parts, fmt.Sprintf("resourceType eq '%s'", rtype))
  }
  if location != "" {
    parts = append(parts, fmt.Sprintf("location eq '%s'", location))
  }
  if tag != "" {
    if eq := strings.Index(tag, "="); eq != -1 {
      parts = append(parts, fmt.Sprintf("tagName eq '%s' and tagValue eq '%s'", tag[:eq], tag[eq+1:]))
    } else {
      parts = append(parts, fmt.Sprintf("tagName eq '%s'", tag))
    }
  }
  return strings.Join(parts, " and ")
}

// genericResourceToMap marshals an armresources.GenericResourceExpanded to the
// shape Python az resource emits (id, name, type, location, tags, etc.).
func genericResourceToMap(r *armresources.GenericResourceExpanded) map[string]interface{} {
  if r == nil {
    return nil
  }
  m := map[string]interface{}{}
  if r.ID != nil { m["id"] = *r.ID }
  if r.Name != nil { m["name"] = *r.Name }
  if r.Type != nil { m["type"] = *r.Type }
  if r.Location != nil { m["location"] = *r.Location }
  if r.Kind != nil { m["kind"] = *r.Kind }
  if r.ManagedBy != nil { m["managedBy"] = *r.ManagedBy }
  if r.Tags != nil {
    tags := map[string]string{}
    for k, v := range r.Tags {
      if v != nil {
        tags[k] = *v
      }
    }
    m["tags"] = tags
  }
  if r.SKU != nil { m["sku"] = r.SKU }
  if r.Identity != nil { m["identity"] = r.Identity }
  if r.Plan != nil { m["plan"] = r.Plan }
  return m
}
