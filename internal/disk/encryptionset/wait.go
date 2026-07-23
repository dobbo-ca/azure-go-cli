package encryptionset

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

type waitOpts struct {
	created bool
	deleted bool
	exists  bool
}

// waitSatisfied reports whether polling should stop given the latest read.
// found is whether the resource currently exists; provisioningState is its
// last-seen provisioning state (empty if not found).
func waitSatisfied(found bool, provisioningState string, o waitOpts) bool {
	if o.deleted {
		return !found
	}
	if !found {
		return false
	}
	if o.exists {
		return true
	}
	// default and --created both wait for a terminal Succeeded state.
	return strings.EqualFold(provisioningState, "Succeeded")
}

func newWaitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait until a disk encryption set reaches a desired state",
		Long:  "Poll a disk encryption set until it is created (provisioning Succeeded), exists, or is deleted.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			o := waitOpts{}
			o.created, _ = cmd.Flags().GetBool("created")
			o.deleted, _ = cmd.Flags().GetBool("deleted")
			o.exists, _ = cmd.Flags().GetBool("exists")
			interval, _ := cmd.Flags().GetInt("interval")
			timeout, _ := cmd.Flags().GetInt("timeout")
			return waitDiskEncryptionSet(context.Background(), cmd, name, resourceGroup, o, interval, timeout)
		},
	}

	cmd.Flags().StringP("name", "n", "", "Disk encryption set name")
	cmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	cmd.Flags().Bool("created", false, "Wait until created (provisioning state Succeeded)")
	cmd.Flags().Bool("deleted", false, "Wait until deleted")
	cmd.Flags().Bool("exists", false, "Wait until the resource exists")
	cmd.Flags().Int("interval", 30, "Polling interval in seconds")
	cmd.Flags().Int("timeout", 3600, "Maximum wait time in seconds")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("resource-group")

	return cmd
}

func waitDiskEncryptionSet(ctx context.Context, cmd *cobra.Command, name, resourceGroup string, o waitOpts, interval, timeout int) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armcompute.NewDiskEncryptionSetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create disk encryption sets client: %w", err)
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
				return fmt.Errorf("failed to get disk encryption set: %w", err)
			}
		} else if resp.Properties != nil && resp.Properties.ProvisioningState != nil {
			state = *resp.Properties.ProvisioningState
		}

		if waitSatisfied(found, state, o) {
			return output.PrintJSON(cmd, map[string]string{"status": "condition met"})
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for disk encryption set '%s' after %d seconds", name, timeout)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
}
