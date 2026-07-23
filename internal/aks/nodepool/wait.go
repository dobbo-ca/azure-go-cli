package nodepool

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

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

func Wait(ctx context.Context, cmd *cobra.Command, clusterName, nodepoolName, resourceGroup string, deleted, exists bool, interval, timeout int) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewAgentPoolsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create agent pools client: %w", err)
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		found := true
		state := ""
		resp, err := client.Get(ctx, resourceGroup, clusterName, nodepoolName, nil)
		if err != nil {
			var respErr *azcore.ResponseError
			if errors.As(err, &respErr) && respErr.StatusCode == 404 {
				found = false
			} else {
				return fmt.Errorf("failed to get node pool: %w", err)
			}
		} else if resp.Properties != nil && resp.Properties.ProvisioningState != nil {
			state = *resp.Properties.ProvisioningState
		}

		if waitDone(found, state, deleted, exists) {
			return output.PrintJSON(cmd, map[string]string{"status": "condition met"})
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for node pool '%s' to reach the desired condition", nodepoolName)
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}
}
