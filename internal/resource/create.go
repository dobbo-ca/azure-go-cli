package resource

import (
  "context"
  "encoding/json"
  "fmt"
  "os"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "create",
    Short: "Create a resource generically from JSON properties",
    RunE:  runCreate,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().String("properties", "", "Resource properties as JSON (or @file.json)")
  cmd.Flags().Bool("is-full-object", false, "Treat --properties as the full request body, not just .properties")
  cmd.Flags().StringP("location", "l", "", "Resource location")
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  cmd.MarkFlagRequired("properties")
  return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  if len(ids) != 1 {
    return fmt.Errorf("create operates on a single resource")
  }
  id := ids[0]

  rawProps, _ := cmd.Flags().GetString("properties")
  isFull, _ := cmd.Flags().GetBool("is-full-object")
  location, _ := cmd.Flags().GetString("location")

  body, err := readJSONInput(rawProps)
  if err != nil {
    return fmt.Errorf("--properties: %w", err)
  }

  var resource armresources.GenericResource
  if isFull {
    raw, _ := json.Marshal(body)
    if err := json.Unmarshal(raw, &resource); err != nil {
      return fmt.Errorf("--properties as full object: %w", err)
    }
  } else {
    resource.Properties = body
    if location != "" {
      resource.Location = &location
    }
  }

  client, cred, sub, err := newGenericClient(cmd)
  if err != nil {
    return err
  }
  _, _, namespace, types, _, perr := ParseResourceID(id)
  if perr != nil {
    return perr
  }
  explicit, _ := cmd.Flags().GetString("api-version")
  preview, _ := cmd.Flags().GetBool("latest-include-preview")
  apiVer, err := azure.ResolveAPIVersion(ctx, cred, sub, namespace, joinTypes(types), explicit, preview)
  if err != nil {
    return err
  }

  poller, err := client.BeginCreateOrUpdateByID(ctx, id, apiVer, resource, nil)
  if err != nil {
    return fmt.Errorf("create %s: %w", id, err)
  }
  resp, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("create %s: %w", id, err)
  }
  return output.PrintJSON(cmd, resp.GenericResource)
}

// readJSONInput parses raw as JSON; if raw begins with '@', reads from the
// referenced file path first.
func readJSONInput(raw string) (interface{}, error) {
  if strings.HasPrefix(raw, "@") {
    data, err := os.ReadFile(raw[1:])
    if err != nil {
      return nil, err
    }
    var v interface{}
    if err := json.Unmarshal(data, &v); err != nil {
      return nil, err
    }
    return v, nil
  }
  var v interface{}
  if err := json.Unmarshal([]byte(raw), &v); err != nil {
    return nil, err
  }
  return v, nil
}
