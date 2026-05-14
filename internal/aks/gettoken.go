package aks

import (
  "context"
  "fmt"
  "os"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/cdobbyn/azure-go-cli/internal/aks/credplugin"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/spf13/cobra"
)

func newGetTokenCmd() *cobra.Command {
  c := &cobra.Command{
    Use:   "get-token",
    Short: "kubectl exec credential plugin (mints AKS bearer tokens)",
    Long: `kubectl exec credential plugin endpoint.

Reads KUBERNETES_EXEC_INFO from the environment, mints an Azure AD access
token for the AKS server-id at the <server-id>/.default scope using this
binary's credential chain (the same chain used by every other 'az' command),
and writes a kubectl ExecCredential JSON object to stdout.

You normally invoke this through kubectl, not directly. See the exec entry
in the kubeconfig produced by 'az aks get-credentials'.`,
    // SilenceErrors so cobra doesn't print the error again with its
    // `Error: ` prefix — kubectl shows whatever lands on stderr.
    SilenceUsage:  true,
    SilenceErrors: true,
    RunE: func(cmd *cobra.Command, args []string) error {
      serverID, _ := cmd.Flags().GetString("server-id")
      tenantID, _ := cmd.Flags().GetString("tenant-id")
      clientID, _ := cmd.Flags().GetString("client-id")

      err := credplugin.GetToken(context.Background(), credplugin.GetTokenOptions{
        ServerID: serverID,
        TenantID: tenantID,
        ClientID: clientID,
        CredentialFactory: func() (azcore.TokenCredential, error) {
          return azure.GetCredential()
        },
        Stdout: os.Stdout,
      })
      if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
      }
      return err
    },
  }
  c.Flags().String("server-id", "", "AAD application ID of the AKS API server (required)")
  c.Flags().String("tenant-id", "", "AAD tenant ID")
  c.Flags().String("client-id", "", "AAD client ID")
  c.MarkFlagRequired("server-id")
  return c
}
