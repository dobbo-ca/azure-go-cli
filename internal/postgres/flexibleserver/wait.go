package flexibleserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// waitDone reports whether polling should stop. found is whether the server
// currently exists; state is its last-seen ServerState. The default (and
// --created) wait for the terminal "Ready" state.
func waitDone(found bool, state string, deleted, exists bool) bool {
	if deleted {
		return !found
	}
	if !found {
		return false
	}
	if exists {
		return true
	}
	return strings.EqualFold(state, "Ready")
}

func Wait(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, deleted, exists bool, interval, timeout int) error {
	client, err := serversClient()
	if err != nil {
		return err
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		found := true
		state := ""
		resp, err := client.Get(ctx, resourceGroup, name, nil)
		if err != nil {
			var respErr *azcore.ResponseError
			if errors.As(err, &respErr) && respErr.StatusCode == 404 {
				found = false
			} else {
				return fmt.Errorf("failed to get server: %w", err)
			}
		} else if resp.Properties != nil && resp.Properties.State != nil {
			state = string(*resp.Properties.State)
		}

		if waitDone(found, state, deleted, exists) {
			return output.PrintJSON(cmd, map[string]string{"status": "condition met"})
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for server '%s' after %d seconds", name, timeout)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
}
