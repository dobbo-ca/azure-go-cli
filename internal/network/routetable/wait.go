package routetable

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// waitDone reports whether polling should stop. found is whether the route table
// currently exists; state is its last-seen provisioning state. The default (and
// --exists) wait for the terminal "Succeeded" state.
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
	return strings.EqualFold(state, "Succeeded")
}

func Wait(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, deleted, exists bool, interval, timeout int) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armnetwork.NewRouteTablesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create route tables client: %w", err)
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
				return fmt.Errorf("failed to get route table: %w", err)
			}
		} else if resp.Properties != nil && resp.Properties.ProvisioningState != nil {
			state = string(*resp.Properties.ProvisioningState)
		}

		if waitDone(found, state, deleted, exists) {
			return output.PrintJSON(cmd, map[string]string{"status": "condition met"})
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for route table '%s' after %d seconds", name, timeout)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
}
