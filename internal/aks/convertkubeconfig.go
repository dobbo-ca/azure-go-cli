package aks

import (
  "fmt"
  "os"
  "path/filepath"

  "github.com/cdobbyn/azure-go-cli/internal/aks/credplugin"
  "github.com/spf13/cobra"
)

func newConvertKubeconfigCmd() *cobra.Command {
  c := &cobra.Command{
    Use:   "convert-kubeconfig",
    Short: "Rewrite an existing kubeconfig to use this binary instead of kubelogin",
    Long: `Rewrite an existing kubeconfig in place, replacing legacy 'auth-provider: azure'
blocks and 'kubelogin' exec entries with exec entries that call this binary's
'az aks get-token' subcommand.

Defaults to ~/.kube/config. The KUBECONFIG env var is intentionally ignored
(kubectl uses it as a merge list, which is ambiguous to rewrite); pass --file
explicitly if you have it set.`,
    SilenceUsage: true,
    RunE: func(cmd *cobra.Command, args []string) error {
      file, _ := cmd.Flags().GetString("file")
      absolute, _ := cmd.Flags().GetBool("absolute-path")

      if file == "" {
        home, err := os.UserHomeDir()
        if err != nil {
          return fmt.Errorf("failed to resolve home directory: %w", err)
        }
        file = filepath.Join(home, ".kube", "config")
      }

      data, err := os.ReadFile(file)
      if err != nil {
        return fmt.Errorf("failed to read %s: %w", file, err)
      }

      out, changed, err := credplugin.Convert(data, credplugin.ConvertOptions{AbsolutePath: absolute})
      if err != nil {
        return err
      }
      if !changed {
        fmt.Fprintf(os.Stderr, "No convertible entries found in %s; nothing to do.\n", file)
        return nil
      }

      if err := os.WriteFile(file, out, 0600); err != nil {
        return fmt.Errorf("failed to write %s: %w", file, err)
      }
      // Ensure kubeconfig permissions are tightened to 0600 regardless of
      // the existing file's mode (os.WriteFile only applies the perm on creation).
      if err := os.Chmod(file, 0600); err != nil {
        return fmt.Errorf("failed to set permissions on %s: %w", file, err)
      }
      fmt.Fprintf(os.Stderr, "Rewrote %s\n", file)
      return nil
    },
  }
  c.Flags().StringP("file", "f", "", "Kubeconfig file to rewrite (default: ~/.kube/config)")
  c.Flags().Bool("absolute-path", false, "Use os.Executable() absolute path instead of 'az' for exec.command")
  return c
}
