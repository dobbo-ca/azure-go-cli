package resource

import (
  "context"
  "encoding/json"
  "fmt"
  "io"
  "net/http"
  "os"
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
  "github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newInvokeActionCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "invoke-action",
    Short: "POST to a resource action endpoint",
    RunE:  runInvokeAction,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().String("action", "", "Action name (e.g. 'restart', 'powerOff')")
  cmd.Flags().String("request-body", "", "JSON body or @file.json")
  cmd.Flags().String("api-version", "", "API version (auto-resolved if not set)")
  cmd.Flags().Bool("latest-include-preview", false, "Include preview versions when auto-resolving --api-version")
  cmd.MarkFlagRequired("action")
  return cmd
}

func runInvokeAction(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  if len(ids) != 1 {
    return fmt.Errorf("invoke-action operates on a single resource")
  }
  id := ids[0]

  action, _ := cmd.Flags().GetString("action")
  rawBody, _ := cmd.Flags().GetString("request-body")

  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }
  sub, err := resolveSubscription(cmd)
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

  client, err := arm.NewClient("github.com/cdobbyn/azure-go-cli/internal/resource", "", cred, nil)
  if err != nil {
    return err
  }
  endpoint := "https://management.azure.com"
  url := fmt.Sprintf("%s%s/%s", endpoint, id, action)

  req, err := runtime.NewRequest(ctx, http.MethodPost, url)
  if err != nil {
    return err
  }
  q := req.Raw().URL.Query()
  q.Set("api-version", apiVer)
  req.Raw().URL.RawQuery = q.Encode()

  if rawBody != "" {
    body, err := readBodyInput(rawBody)
    if err != nil {
      return err
    }
    if err := req.SetBody(body, "application/json"); err != nil {
      return err
    }
  }

  resp, err := client.Pipeline().Do(req)
  if err != nil {
    return fmt.Errorf("invoke-action %s: %w", action, err)
  }
  defer resp.Body.Close()
  data, _ := io.ReadAll(resp.Body)

  if resp.StatusCode >= 400 {
    return fmt.Errorf("invoke-action %s: %s: %s", action, resp.Status, string(data))
  }

  if len(data) == 0 {
    if resp.StatusCode == http.StatusAccepted {
      asyncURL := resp.Header.Get("Azure-AsyncOperation")
      if asyncURL == "" {
        asyncURL = resp.Header.Get("Location")
      }
      fmt.Fprintf(cmd.OutOrStdout(), "Action %s accepted (status %s); operation may still be running\n", action, resp.Status)
      if asyncURL != "" {
        fmt.Fprintf(cmd.OutOrStdout(), "Async operation: %s\n", asyncURL)
      }
      return nil
    }
    fmt.Fprintf(cmd.OutOrStdout(), "Action %s completed (status %s)\n", action, resp.Status)
    return nil
  }
  var parsed interface{}
  if err := json.Unmarshal(data, &parsed); err != nil {
    fmt.Fprintln(cmd.OutOrStdout(), string(data))
    return nil
  }
  return output.PrintJSON(cmd, parsed)
}

// readBodyInput returns an io.ReadSeekCloser usable by req.SetBody, accepting
// either a literal JSON string or @path/to/file.json.
func readBodyInput(raw string) (io.ReadSeekCloser, error) {
  if strings.HasPrefix(raw, "@") {
    f, err := os.Open(raw[1:])
    if err != nil {
      return nil, err
    }
    return f, nil
  }
  return nopReadSeekCloser{strings.NewReader(raw)}, nil
}

type nopReadSeekCloser struct{ *strings.Reader }

func (nopReadSeekCloser) Close() error { return nil }
