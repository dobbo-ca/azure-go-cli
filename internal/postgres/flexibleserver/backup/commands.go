package backup

import (
	"github.com/spf13/cobra"
)

func NewBackupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage on-demand backups for a PostgreSQL flexible server",
		Long:  "Create, list, show, and delete on-demand backups for an Azure Database for PostgreSQL flexible server. Automated PITR backups are managed by the service and exposed via list/show.",
	}
	return cmd
}
