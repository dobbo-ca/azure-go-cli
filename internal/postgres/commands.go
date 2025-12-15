package postgres

import (
	"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver"
	"github.com/spf13/cobra"
)

func NewPostgresCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "postgres",
		Short: "Manage Azure Database for PostgreSQL",
		Long:  "Commands to manage Azure Database for PostgreSQL servers",
	}

	cmd.AddCommand(flexibleserver.NewFlexibleServerCommand())
	return cmd
}
