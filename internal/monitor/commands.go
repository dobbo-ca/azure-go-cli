package monitor

import (
	"github.com/cdobbyn/azure-go-cli/internal/monitor/actiongroup"
	"github.com/cdobbyn/azure-go-cli/internal/monitor/activitylog"
	"github.com/cdobbyn/azure-go-cli/internal/monitor/autoscale"
	"github.com/cdobbyn/azure-go-cli/internal/monitor/diagnosticsettings"
	"github.com/cdobbyn/azure-go-cli/internal/monitor/logprofiles"
	"github.com/cdobbyn/azure-go-cli/internal/monitor/metrics"
	"github.com/spf13/cobra"
)

func NewMonitorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Manage Azure Monitor resources",
		Long:  "Commands to manage Azure Monitor action groups, autoscale, diagnostic settings, log profiles, metrics, and activity logs",
	}

	cmd.AddCommand(
		actiongroup.NewActionGroupCommand(),
		autoscale.NewAutoscaleCommand(),
		diagnosticsettings.NewDiagnosticSettingsCommand(),
		logprofiles.NewLogProfilesCommand(),
		metrics.NewMetricsCommand(),
		activitylog.NewActivityLogCommand(),
	)
	return cmd
}
