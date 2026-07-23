package flexibleserver

import (
	"context"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Stop(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, noWait bool) error {
	client, err := serversClient()
	if err != nil {
		return err
	}

	fmt.Printf("Stopping PostgreSQL flexible server '%s'...\n", name)
	poller, err := client.BeginStop(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin stop: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Stop of server '%s' started.", name)})
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}
	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("Server '%s' stopped.", name)})
}
