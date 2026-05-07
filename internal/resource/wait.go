package resource

import (
  "context"
  "encoding/json"
  "fmt"
  "strings"
  "time"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/jmespath/go-jmespath"
  "github.com/spf13/cobra"
)

func newWaitCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "wait",
    Short: "Wait until a resource reaches a desired condition",
    RunE:  runWait,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().Bool("created", false, "Wait until the resource exists")
  cmd.Flags().Bool("deleted", false, "Wait until the resource no longer exists")
  cmd.Flags().Bool("updated", false, "Wait until provisioningState reaches a terminal state")
  cmd.Flags().Bool("exists", false, "Alias for --created")
  cmd.Flags().String("custom", "", "Custom JMESPath query that must evaluate truthy on the resource body")
  cmd.Flags().Int("interval", 30, "Polling interval in seconds")
  cmd.Flags().Int("timeout", 3600, "Timeout in seconds")
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  return cmd
}

func runWait(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  if len(ids) != 1 {
    return fmt.Errorf("wait operates on a single resource; pass exactly one --ids or use name-mode flags")
  }
  id := ids[0]

  created, _ := cmd.Flags().GetBool("created")
  deleted, _ := cmd.Flags().GetBool("deleted")
  updated, _ := cmd.Flags().GetBool("updated")
  exists, _ := cmd.Flags().GetBool("exists")
  custom, _ := cmd.Flags().GetString("custom")
  interval, _ := cmd.Flags().GetInt("interval")
  timeout, _ := cmd.Flags().GetInt("timeout")

  conditions := 0
  for _, c := range []bool{created, deleted, updated, exists, custom != ""} {
    if c {
      conditions++
    }
  }
  if conditions != 1 {
    return fmt.Errorf("specify exactly one of --created, --deleted, --updated, --exists, --custom")
  }

  client, cred, sub, err := newGenericClient(cmd)
  if err != nil {
    return err
  }
  explicit, _ := cmd.Flags().GetString("api-version")
  preview, _ := cmd.Flags().GetBool("latest-include-preview")

  _, _, namespace, types, _, perr := ParseResourceID(id)
  if perr != nil {
    return perr
  }
  apiVer, err := azure.ResolveAPIVersion(ctx, cred, sub, namespace, joinTypes(types), explicit, preview)
  if err != nil {
    return err
  }

  deadline := time.Now().Add(time.Duration(timeout) * time.Second)
  for {
    if time.Now().After(deadline) {
      return fmt.Errorf("timed out waiting for %s", id)
    }

    resp, getErr := client.GetByID(ctx, id, apiVer, nil)
    notFound := getErr != nil && isNotFound(getErr)

    switch {
    case deleted:
      if notFound {
        return nil
      }
    case created || exists:
      if getErr == nil {
        return nil
      }
    case updated:
      if getErr == nil {
        if state := provisioningState(resp.GenericResource); isTerminal(state) {
          return nil
        }
      }
    case custom != "":
      if getErr == nil {
        body, _ := json.Marshal(resp.GenericResource)
        var parsed interface{}
        json.Unmarshal(body, &parsed)
        result, jerr := jmespath.Search(custom, parsed)
        if jerr != nil {
          return fmt.Errorf("--custom JMESPath: %w", jerr)
        }
        if isTruthy(result) {
          return nil
        }
      }
    }

    if getErr != nil && !notFound {
      return getErr
    }

    time.Sleep(time.Duration(interval) * time.Second)
  }
}

func provisioningState(r armresources.GenericResource) string {
  if r.Properties == nil {
    return ""
  }
  m, ok := r.Properties.(map[string]interface{})
  if !ok {
    return ""
  }
  if s, ok := m["provisioningState"].(string); ok {
    return s
  }
  return ""
}

func isTerminal(state string) bool {
  switch state {
  case "Succeeded", "Failed", "Canceled":
    return true
  }
  return false
}

func isNotFound(err error) bool {
  if err == nil {
    return false
  }
  msg := err.Error()
  return strings.Contains(msg, "ResourceNotFound") || strings.Contains(msg, "404")
}

func isTruthy(v interface{}) bool {
  switch t := v.(type) {
  case nil:
    return false
  case bool:
    return t
  case string:
    return t != ""
  case float64:
    return t != 0
  case []interface{}:
    return len(t) > 0
  case map[string]interface{}:
    return len(t) > 0
  }
  return true
}
