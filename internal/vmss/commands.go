package vmss

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/vmss/extension"
	"github.com/cdobbyn/azure-go-cli/internal/vmss/identity"
	"github.com/cdobbyn/azure-go-cli/internal/vmss/nic"
	"github.com/cdobbyn/azure-go-cli/internal/vmss/rollingupgrade"
	"github.com/cdobbyn/azure-go-cli/internal/vmss/runcommand"
	"github.com/spf13/cobra"
)

func NewVmssCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vmss",
		Short: "Manage virtual machine scale sets",
		Long:  "Commands to manage Azure virtual machine scale sets and related resources",
	}

	// --- read-only ---
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List virtual machine scale sets",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, rg)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a virtual machine scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), cmd, rg, name)
		},
	}
	addRGName(showCmd)

	instanceViewCmd := &cobra.Command{
		Use:   "get-instance-view",
		Short: "Get the instance view of a virtual machine scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return GetInstanceView(context.Background(), cmd, rg, name)
		},
	}
	addRGName(instanceViewCmd)

	osUpgradeHistoryCmd := &cobra.Command{
		Use:   "get-os-upgrade-history",
		Short: "Get the OS upgrade history of a virtual machine scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return GetOSUpgradeHistory(context.Background(), cmd, rg, name)
		},
	}
	addRGName(osUpgradeHistoryCmd)

	listInstancesCmd := &cobra.Command{
		Use:   "list-instances",
		Short: "List the VM instances in a virtual machine scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return ListInstances(context.Background(), cmd, rg, name)
		},
	}
	addRGName(listInstancesCmd)

	listSkusCmd := &cobra.Command{
		Use:   "list-skus",
		Short: "List the SKUs available for a virtual machine scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return ListSKUs(context.Background(), cmd, rg, name)
		},
	}
	addRGName(listSkusCmd)

	listInstancePublicIPsCmd := &cobra.Command{
		Use:   "list-instance-public-ips",
		Short: "List the public IP addresses of VM instances in a scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return ListInstancePublicIPs(context.Background(), cmd, rg, name)
		},
	}
	addRGName(listInstancePublicIPsCmd)

	updateDomainWalkCmd := &cobra.Command{
		Use:   "update-domain-walk",
		Short: "Start a manual platform update domain walk (Service Fabric recovery)",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			pud, _ := cmd.Flags().GetInt32("platform-update-domain")
			return UpdateDomainWalk(context.Background(), cmd, rg, name, pud)
		},
	}
	addRGName(updateDomainWalkCmd)
	updateDomainWalkCmd.Flags().Int32("platform-update-domain", 0, "Platform update domain for which a manual recovery walk is requested")

	// --- LRO power/lifecycle ---
	deleteCmd := newLRO("delete", "Delete a virtual machine scale set", Delete)
	startCmd := newLRO("start", "Start the VMs in a virtual machine scale set", Start)
	stopCmd := newLRO("stop", "Power off (stop) the VMs in a virtual machine scale set", Stop)
	restartCmd := newLRO("restart", "Restart the VMs in a virtual machine scale set", Restart)
	deallocateCmd := newLRO("deallocate", "Deallocate the VMs in a virtual machine scale set", Deallocate)
	reimageCmd := newLRO("reimage", "Reimage the VMs in a virtual machine scale set", Reimage)
	performMaintenanceCmd := newLRO("perform-maintenance", "Perform maintenance on the VMs in a scale set", PerformMaintenance)

	// scale
	scaleCmd := &cobra.Command{
		Use:   "scale",
		Short: "Change the number of VMs in a virtual machine scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			capacity, _ := cmd.Flags().GetInt64("new-capacity")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Scale(context.Background(), cmd, rg, name, capacity, noWait)
		},
	}
	addRGName(scaleCmd)
	scaleCmd.Flags().Int64("new-capacity", 0, "New number of VM instances")
	scaleCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	scaleCmd.MarkFlagRequired("new-capacity")

	// delete-instances / update-instances
	deleteInstancesCmd := newInstanceIDsCmd("delete-instances", "Delete specific VM instances from a scale set", DeleteInstances)
	updateInstancesCmd := newInstanceIDsCmd("update-instances", "Upgrade specific VM instances to the latest scale set model", UpdateInstances)

	// simulate-eviction
	simulateEvictionCmd := &cobra.Command{
		Use:   "simulate-eviction",
		Short: "Simulate the eviction of a Spot VM in a scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			instanceID, _ := cmd.Flags().GetString("instance-id")
			return SimulateEviction(context.Background(), cmd, rg, name, instanceID)
		},
	}
	addRGName(simulateEvictionCmd)
	simulateEvictionCmd.Flags().String("instance-id", "", "VM instance ID")
	simulateEvictionCmd.MarkFlagRequired("instance-id")

	// set-orchestration-service-state
	orchStateCmd := &cobra.Command{
		Use:   "set-orchestration-service-state",
		Short: "Change the state of an orchestration service on a scale set",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			service, _ := cmd.Flags().GetString("service-name")
			action, _ := cmd.Flags().GetString("action")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return SetOrchestrationServiceState(context.Background(), cmd, rg, name, service, action, noWait)
		},
	}
	addRGName(orchStateCmd)
	orchStateCmd.Flags().String("service-name", "AutomaticRepairs", "Orchestration service name")
	orchStateCmd.Flags().String("action", "", "Action to perform (Resume or Suspend)")
	orchStateCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	orchStateCmd.MarkFlagRequired("action")

	// wait
	waitCmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait until a condition of the virtual machine scale set is met",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			deleted, _ := cmd.Flags().GetBool("deleted")
			exists, _ := cmd.Flags().GetBool("exists")
			interval, _ := cmd.Flags().GetInt("interval")
			timeout, _ := cmd.Flags().GetInt("timeout")
			return Wait(context.Background(), cmd, name, rg, deleted, exists, interval, timeout)
		},
	}
	addRGName(waitCmd)
	waitCmd.Flags().Bool("deleted", false, "Wait until deleted")
	waitCmd.Flags().Bool("exists", false, "Wait until the resource exists")
	waitCmd.Flags().Int("interval", 30, "Polling interval in seconds")
	waitCmd.Flags().Int("timeout", 3600, "Maximum wait time in seconds")

	cmd.AddCommand(
		listCmd, showCmd, instanceViewCmd, osUpgradeHistoryCmd, listInstancesCmd, listSkusCmd,
		listInstancePublicIPsCmd, updateDomainWalkCmd,
		deleteCmd, startCmd, stopCmd, restartCmd, deallocateCmd, reimageCmd, performMaintenanceCmd,
		scaleCmd, deleteInstancesCmd, updateInstancesCmd, simulateEvictionCmd, orchStateCmd, waitCmd,
		identity.NewIdentityCommand(),
		extension.NewExtensionCommand(),
		rollingupgrade.NewRollingUpgradeCommand(),
		nic.NewNicCommand(),
		runcommand.NewRunCommandCommand(),
	)
	return cmd
}

// addRGName adds the standard required --resource-group and --name flags.
func addRGName(c *cobra.Command) {
	c.Flags().StringP("resource-group", "g", "", "Resource group name")
	c.Flags().StringP("name", "n", "", "Scale set name")
	c.MarkFlagRequired("resource-group")
	c.MarkFlagRequired("name")
}

// newLRO builds a standard LRO subcommand taking (rg, name, noWait).
func newLRO(use, short string, run func(context.Context, *cobra.Command, string, string, bool) error) *cobra.Command {
	c := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return run(context.Background(), cmd, rg, name, noWait)
		},
	}
	addRGName(c)
	c.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	return c
}

// newInstanceIDsCmd builds a subcommand taking (rg, name, instanceIDs, noWait).
func newInstanceIDsCmd(use, short string, run func(context.Context, *cobra.Command, string, string, []string, bool) error) *cobra.Command {
	c := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			ids, _ := cmd.Flags().GetStringSlice("instance-ids")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return run(context.Background(), cmd, rg, name, ids, noWait)
		},
	}
	addRGName(c)
	c.Flags().StringSlice("instance-ids", nil, "VM instance IDs")
	c.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	c.MarkFlagRequired("instance-ids")
	return c
}
