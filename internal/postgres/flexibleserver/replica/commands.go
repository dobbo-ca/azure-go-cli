package replica

import (
	"context"

	"github.com/spf13/cobra"
)

// NewReplicaCommand builds the "replica" command tree for managing read
// replicas of a PostgreSQL flexible server.
func NewReplicaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replica",
		Short: "Manage read replicas for a PostgreSQL flexible server",
	}

	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newPromoteCommand())
	cmd.AddCommand(newStopReplicationCommand())

	return cmd
}

func newListCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "List the read replicas of a source flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			return List(context.Background(), cmd, rg, server)
		},
	}
	c.Flags().StringP("resource-group", "g", "", "Resource group name")
	c.Flags().String("server-name", "", "Source (primary) flexible server name")
	c.MarkFlagRequired("resource-group")
	c.MarkFlagRequired("server-name")
	return c
}

func newCreateCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a read replica of a source flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			replicaName, _ := cmd.Flags().GetString("replica-name")
			sourceServer, _ := cmd.Flags().GetString("source-server")
			location, _ := cmd.Flags().GetString("location")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Create(context.Background(), cmd, rg, replicaName, sourceServer, location, noWait)
		},
	}
	c.Flags().StringP("resource-group", "g", "", "Resource group for the new replica")
	c.Flags().StringP("replica-name", "n", "", "Name of the new read replica")
	c.Flags().String("source-server", "", "Full resource ID of the source (primary) server")
	c.Flags().StringP("location", "l", "", "Azure region for the new replica")
	c.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	c.MarkFlagRequired("resource-group")
	c.MarkFlagRequired("replica-name")
	c.MarkFlagRequired("source-server")
	return c
}

func newPromoteCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "promote",
		Short: "Promote a read replica to a standalone server or new primary",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			promoteMode, _ := cmd.Flags().GetString("promote-mode")
			promoteOption, _ := cmd.Flags().GetString("promote-option")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Promote(context.Background(), cmd, rg, server, promoteMode, promoteOption, noWait)
		},
	}
	c.Flags().StringP("resource-group", "g", "", "Resource group name")
	c.Flags().String("server-name", "", "Read replica server name")
	c.Flags().String("promote-mode", "standalone", "Promote mode: standalone or switchover")
	c.Flags().String("promote-option", "planned", "Promote option: planned or forced")
	c.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	c.MarkFlagRequired("resource-group")
	c.MarkFlagRequired("server-name")
	return c
}

func newStopReplicationCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "stop-replication",
		Short: "Stop replication on a read replica, making it an independent server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return StopReplication(context.Background(), cmd, rg, server, noWait)
		},
	}
	c.Flags().StringP("resource-group", "g", "", "Resource group name")
	c.Flags().String("server-name", "", "Read replica server name")
	c.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	c.MarkFlagRequired("resource-group")
	c.MarkFlagRequired("server-name")
	return c
}
