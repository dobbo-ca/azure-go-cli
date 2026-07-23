package extension

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Set(ctx context.Context, cmd *cobra.Command, resourceGroup, vmName, name, publisher, extType, version, settingsJSON, location string, autoUpgradeMinor, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armcompute.NewVirtualMachineExtensionsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create vm extensions client: %w", err)
	}

	if location == "" {
		vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
		if err != nil {
			return fmt.Errorf("failed to create vm client: %w", err)
		}
		vm, err := vmClient.Get(ctx, resourceGroup, vmName, nil)
		if err != nil {
			return fmt.Errorf("failed to get vm: %w", err)
		}
		if vm.Location != nil {
			location = *vm.Location
		}
	}

	ext := armcompute.VirtualMachineExtension{
		Location: to.Ptr(location),
		Properties: &armcompute.VirtualMachineExtensionProperties{
			Publisher:               to.Ptr(publisher),
			Type:                    to.Ptr(extType),
			TypeHandlerVersion:      to.Ptr(version),
			AutoUpgradeMinorVersion: to.Ptr(autoUpgradeMinor),
		},
	}
	if settingsJSON != "" {
		var settings any
		if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
			return fmt.Errorf("invalid --settings JSON: %w", err)
		}
		ext.Properties.Settings = settings
	}

	fmt.Printf("Setting extension '%s' on VM '%s'...\n", name, vmName)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, vmName, name, ext, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create or update: %w", err)
	}
	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "set started"})
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("set failed: %w", err)
	}
	return output.PrintJSON(cmd, resp.VirtualMachineExtension)
}
