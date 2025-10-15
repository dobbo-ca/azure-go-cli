package aks

import (
  "context"
  "fmt"
  "os"
  "path/filepath"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/cdobbyn/azure-go-cli/pkg/kubeconfig"
)

type GetCredentialsOptions struct {
  ClusterName   string
  ResourceGroup string
  Admin         bool
  File          string
  Overwrite     bool
  Context       string
}

func GetCredentials(ctx context.Context, opts GetCredentialsOptions) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create AKS client: %w", err)
  }

  var kubeConfig []byte
  if opts.Admin {
    fmt.Fprintf(os.Stderr, "Getting admin credentials...\n")
    resp, err := client.ListClusterAdminCredentials(ctx, opts.ResourceGroup, opts.ClusterName, nil)
    if err != nil {
      return fmt.Errorf("failed to get admin credentials: %w", err)
    }
    if len(resp.Kubeconfigs) == 0 {
      return fmt.Errorf("no kubeconfig found")
    }
    kubeConfig = resp.Kubeconfigs[0].Value
  } else {
    fmt.Fprintf(os.Stderr, "Getting user credentials...\n")
    resp, err := client.ListClusterUserCredentials(ctx, opts.ResourceGroup, opts.ClusterName, nil)
    if err != nil {
      return fmt.Errorf("failed to get user credentials: %w", err)
    }
    if len(resp.Kubeconfigs) == 0 {
      return fmt.Errorf("no kubeconfig found")
    }
    kubeConfig = resp.Kubeconfigs[0].Value
  }

  if kubeConfig == nil {
    return fmt.Errorf("no kubeconfig data returned")
  }

  // If context is specified but file is "-", output to stdout with context name updated
  if opts.Context != "" && opts.File == "-" {
    kubeConfig, err = kubeconfig.UpdateContext(kubeConfig, opts.Context)
    if err != nil {
      return fmt.Errorf("failed to update context: %w", err)
    }
  }

  // Output to stdout
  if opts.File == "-" {
    fmt.Print(string(kubeConfig))
    return nil
  }

  // Determine output file
  file := opts.File
  if file == "" {
    home, err := os.UserHomeDir()
    if err != nil {
      return fmt.Errorf("failed to get home directory: %w", err)
    }
    file = filepath.Join(home, ".kube", "config")
  }

  // Write or merge kubeconfig
  if opts.Overwrite {
    if err := os.WriteFile(file, kubeConfig, 0600); err != nil {
      return fmt.Errorf("failed to write kubeconfig: %w", err)
    }
    fmt.Fprintf(os.Stderr, "Saved kubeconfig to %s\n", file)
  } else {
    if err := kubeconfig.Merge(file, kubeConfig); err != nil {
      return fmt.Errorf("failed to merge kubeconfig: %w", err)
    }
    fmt.Fprintf(os.Stderr, "Merged \"%s\" as current context in %s\n", opts.ClusterName, file)
  }

  return nil
}
