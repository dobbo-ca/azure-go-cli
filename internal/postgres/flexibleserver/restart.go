package flexibleserver

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Restart(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, noWait bool) error {
	client, err := serversClient()
	if err != nil {
		return err
	}

	fmt.Printf("Restarting PostgreSQL flexible server '%s'...\n", name)
	poller, err := client.BeginRestart(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin restart: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Restart of server '%s' started.", name)})
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to restart server: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Server '%s' restarted.", name)})
}
