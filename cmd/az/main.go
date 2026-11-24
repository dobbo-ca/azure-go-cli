package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cdobbyn/azure-go-cli/internal/account"
	"github.com/cdobbyn/azure-go-cli/internal/aks"
	"github.com/cdobbyn/azure-go-cli/internal/auth"
	"github.com/cdobbyn/azure-go-cli/internal/dataprotection"
	"github.com/cdobbyn/azure-go-cli/internal/devops"
	"github.com/cdobbyn/azure-go-cli/internal/devops/boards"
	"github.com/cdobbyn/azure-go-cli/internal/devops/pipelines"
	"github.com/cdobbyn/azure-go-cli/internal/devops/repos"
	"github.com/cdobbyn/azure-go-cli/internal/disk"
	"github.com/cdobbyn/azure-go-cli/internal/disk/encryptionset"
	"github.com/cdobbyn/azure-go-cli/internal/feature"
	"github.com/cdobbyn/azure-go-cli/internal/group"
	"github.com/cdobbyn/azure-go-cli/internal/hdinsight"
	"github.com/cdobbyn/azure-go-cli/internal/identity"
	"github.com/cdobbyn/azure-go-cli/internal/keyvault"
	"github.com/cdobbyn/azure-go-cli/internal/lock"
	"github.com/cdobbyn/azure-go-cli/internal/monitor"
	"github.com/cdobbyn/azure-go-cli/internal/network"
	"github.com/cdobbyn/azure-go-cli/internal/pim"
	"github.com/cdobbyn/azure-go-cli/internal/postgres"
	"github.com/cdobbyn/azure-go-cli/internal/provider"
	"github.com/cdobbyn/azure-go-cli/internal/quota"
	"github.com/cdobbyn/azure-go-cli/internal/resource"
	"github.com/cdobbyn/azure-go-cli/internal/role"
	"github.com/cdobbyn/azure-go-cli/internal/sig"
	"github.com/cdobbyn/azure-go-cli/internal/ssh"
	"github.com/cdobbyn/azure-go-cli/internal/storage"
	"github.com/cdobbyn/azure-go-cli/internal/vm"
	"github.com/cdobbyn/azure-go-cli/internal/vmss"
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
	os.Exit(run(newRootCmd(), os.Args[1:], os.Stderr))
}

func newRootCmd() *cobra.Command {
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
		boards.NewBoardsCommand(),
		dataprotection.NewDataProtectionCommand(),
		devops.NewDevOpsCommand(),
		disk.NewDiskCommand(),
		encryptionset.NewEncryptionSetCommand(),
		feature.NewFeatureCommand(),
		group.NewGroupCommand(),
		hdinsight.NewHDInsightCommand(),
		identity.NewIdentityCommand(),
		lock.NewLockCommand(),
		monitor.NewMonitorCommand(),
		network.NewNetworkCommand(),
		pim.NewPIMCommand(),
		pipelines.NewPipelinesCommand(),
		storage.NewStorageCommand(),
		postgres.NewPostgresCommand(),
		provider.NewProviderCommand(),
		keyvault.NewKeyVaultCommand(),
		quota.NewQuotaCommand(),
		repos.NewReposCommand(),
		resource.NewResourceCommand(),
		role.NewRoleCmd(),
		sig.NewSigCommand(),
		ssh.NewSSHCommand(),
		vm.NewVMCommand(),
		vmss.NewVmssCommand(),
	)

	return rootCmd
}

// runError marks an error returned by a command's RunE, as opposed to a
// flag, argument, or unknown-command error raised by cobra itself.
type runError struct{ err error }

func (e runError) Error() string { return e.err.Error() }
func (e runError) Unwrap() error { return e.err }

// markRunErrors wraps every RunE in the tree so run can tell a command's own
// failure apart from a usage mistake.
func markRunErrors(c *cobra.Command) {
	if runE := c.RunE; runE != nil {
		c.RunE = func(cmd *cobra.Command, args []string) error {
			if err := runE(cmd, args); err != nil {
				return runError{err}
			}
			return nil
		}
	}
	for _, sub := range c.Commands() {
		markRunErrors(sub)
	}
}

// run executes root and prints any error exactly once. Cobra's own error and
// usage printing is silenced; usage is shown only for usage mistakes (bad
// flags, bad arguments, missing required flags), never for runtime errors.
func run(root *cobra.Command, args []string, stderr io.Writer) int {
	root.SilenceErrors = true
	root.SilenceUsage = true
	markRunErrors(root)
	root.SetArgs(args)

	cmd, err := root.ExecuteC()
	if err == nil {
		return 0
	}
	fmt.Fprintf(stderr, "Error: %v\n", err)
	var re runError
	if !errors.As(err, &re) && cmd != nil {
		fmt.Fprint(stderr, cmd.UsageString())
	}
	return 1
}
