package flexibleserver

import (
	"fmt"
	"strings"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// buildConnectionStrings returns ready-to-use connection strings for common
// PostgreSQL clients. It performs no network calls — it only formats the
// provided values. Missing user/password/database are left as placeholders so
// the output is still copy-paste useful.
func buildConnectionStrings(serverName, database, user, password string) map[string]string {
	host := serverName
	if !strings.Contains(host, ".") {
		host = serverName + ".postgres.database.azure.com"
	}
	const port = "5432"

	return map[string]string{
		"psql":    fmt.Sprintf("psql \"host=%s port=%s dbname=%s user=%s password=%s sslmode=require\"", host, port, database, user, password),
		"uri":     fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=require", user, password, host, port, database),
		"jdbc":    fmt.Sprintf("jdbc:postgresql://%s:%s/%s?user=%s&password=%s&sslmode=require", host, port, database, user, password),
		"ado.net": fmt.Sprintf("Server=%s;Database=%s;Port=%s;User Id=%s;Password=%s;Ssl Mode=Require;", host, database, port, user, password),
		"node.js": fmt.Sprintf("postgres://%s:%s@%s:%s/%s?ssl=true", user, password, host, port, database),
		"python":  fmt.Sprintf("dbname=%s user=%s host=%s password=%s port=%s sslmode=require", database, user, host, password, port),
		"php":     fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=require", host, port, database, user, password),
		"ruby":    fmt.Sprintf("host=%s; dbname=%s; user=%s; password=%s; port=%s; sslmode=require", host, database, user, password, port),
	}
}

func ShowConnectionString(cmd *cobra.Command, serverName, database, user, password string) error {
	return output.PrintJSON(cmd, buildConnectionStrings(serverName, database, user, password))
}

func newShowConnectionStringCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-connection-string",
		Short: "Show connection strings for a PostgreSQL flexible server",
		Long:  "Print ready-to-use connection strings for common PostgreSQL clients. This command does not contact Azure; unspecified values are left as placeholders.",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverName, _ := cmd.Flags().GetString("server-name")
			database, _ := cmd.Flags().GetString("database-name")
			user, _ := cmd.Flags().GetString("admin-user")
			password, _ := cmd.Flags().GetString("admin-password")
			return ShowConnectionString(cmd, serverName, database, user, password)
		},
	}
	cmd.Flags().StringP("server-name", "n", "", "Server name")
	cmd.Flags().StringP("database-name", "d", "postgres", "Database name")
	cmd.Flags().StringP("admin-user", "u", "{username}", "Administrator username")
	cmd.Flags().StringP("admin-password", "p", "{password}", "Administrator password")
	cmd.MarkFlagRequired("server-name")
	return cmd
}
