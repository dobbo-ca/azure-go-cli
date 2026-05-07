package main

import (
	"fmt"
	"os"

	"github.com/cdobbyn/azure-go-cli/internal/account"
	"github.com/cdobbyn/azure-go-cli/internal/aks"
	"github.com/cdobbyn/azure-go-cli/internal/auth"
	"github.com/cdobbyn/azure-go-cli/internal/dataprotection"
	"github.com/cdobbyn/azure-go-cli/internal/disk"
	"github.com/cdobbyn/azure-go-cli/internal/disk/encryptionset"
	"github.com/cdobbyn/azure-go-cli/internal/feature"
	"github.com/cdobbyn/azure-go-cli/internal/group"
	"github.com/cdobbyn/azure-go-cli/internal/identity"
	"github.com/cdobbyn/azure-go-cli/internal/keyvault"
	"github.com/cdobbyn/azure-go-cli/internal/network"
	"github.com/cdobbyn/azure-go-cli/internal/postgres"
	"github.com/cdobbyn/azure-go-cli/internal/quota"
	"github.com/cdobbyn/azure-go-cli/internal/resource"
	"github.com/cdobbyn/azure-go-cli/internal/role"
	"github.com/cdobbyn/azure-go-cli/internal/storage"
	"github.com/cdobbyn/azure-go-cli/internal/vm"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// Set via ldflags at build time
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "az",
		Short: "Azure CLI implemented in Go",
		Long:  "A lightweight Azure CLI implementation in Go with core authentication and management commands",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Enable debug logging if --debug flag is set
			debug, _ := cmd.Flags().GetBool("debug")
			if debug {
				logger.EnableDebug()
			}
		},
	}

	// Add global flags
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().String("subscription", "", "Subscription ID or name (overrides default)")
	rootCmd.PersistentFlags().StringP("output", "o", "json", "Output format (json, table, tsv, yaml, none)")
	rootCmd.PersistentFlags().String("query", "", "JMESPath query string to filter output")

	// Add version command (required by Terraform azurerm provider)
	// The azurerm provider parses "azure-cli" and enforces a minimum version,
	// so we report a compatible version while exposing our real version separately.
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show the version of the Azure CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(`{"azure-cli": "2.67.0", "azure-cli-go": "%s", "azure-cli-go-commit": "%s", "azure-cli-go-build-date": "%s"}`, version, commit, date)
			fmt.Println()
			return nil
		},
	})

	// Add all domain commands
	rootCmd.AddCommand(
		auth.NewLoginCommand(),
		auth.NewLogoutCommand(),
		account.NewAccountCommand(),
		aks.NewAKSCommand(),
		dataprotection.NewDataProtectionCommand(),
		disk.NewDiskCommand(),
		encryptionset.NewEncryptionSetCommand(),
		feature.NewFeatureCommand(),
		group.NewGroupCommand(),
		identity.NewIdentityCommand(),
		network.NewNetworkCommand(),
		storage.NewStorageCommand(),
		postgres.NewPostgresCommand(),
		keyvault.NewKeyVaultCommand(),
		quota.NewQuotaCommand(),
		resource.NewResourceCommand(),
		role.NewRoleCmd(),
		vm.NewVMCommand(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
