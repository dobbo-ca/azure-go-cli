package keyvault

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func privateEndpointClient() (*armkeyvault.PrivateEndpointConnectionsClient, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return nil, err
	}
	client, err := armkeyvault.NewPrivateEndpointConnectionsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create private endpoint connections client: %w", err)
	}
	return client, nil
}

// SetPrivateEndpointConnectionStatus approves or rejects a connection request,
// as _update_private_endpoint_connection_status does (custom.py:1907).
func SetPrivateEndpointConnectionStatus(ctx context.Context, cmd *cobra.Command, vaultName, resourceGroup, name, description string, approved bool) error {
	client, err := privateEndpointClient()
	if err != nil {
		return err
	}
	current, err := client.Get(ctx, resourceGroup, vaultName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get the private endpoint connection: %w", err)
	}
	status := armkeyvault.PrivateEndpointServiceConnectionStatusRejected
	if approved {
		status = armkeyvault.PrivateEndpointServiceConnectionStatusApproved
	}
	connection := current.PrivateEndpointConnection
	if connection.Properties == nil {
		connection.Properties = &armkeyvault.PrivateEndpointConnectionProperties{}
	}
	connection.Properties.PrivateLinkServiceConnectionState = &armkeyvault.PrivateLinkServiceConnectionState{
		Status: to.Ptr(status),
	}
	if description != "" {
		connection.Properties.PrivateLinkServiceConnectionState.Description = to.Ptr(description)
	}
	resp, err := client.Put(ctx, resourceGroup, vaultName, name, connection, nil)
	if err != nil {
		return fmt.Errorf("failed to update the private endpoint connection: %w", err)
	}
	return output.PrintJSON(cmd, resp.PrivateEndpointConnection)
}

func ShowPrivateEndpointConnection(ctx context.Context, cmd *cobra.Command, vaultName, resourceGroup, name string) error {
	client, err := privateEndpointClient()
	if err != nil {
		return err
	}
	resp, err := client.Get(ctx, resourceGroup, vaultName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get the private endpoint connection: %w", err)
	}
	return output.PrintJSON(cmd, resp.PrivateEndpointConnection)
}

func DeletePrivateEndpointConnection(ctx context.Context, cmd *cobra.Command, vaultName, resourceGroup, name string) error {
	client, err := privateEndpointClient()
	if err != nil {
		return err
	}
	poller, err := client.BeginDelete(ctx, resourceGroup, vaultName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to delete the private endpoint connection: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete the private endpoint connection: %w", err)
	}
	return output.PrintJSON(cmd, resp.PrivateEndpointConnection)
}

// ListPrivateEndpointConnections lists the connections of a vault. azure-cli's
// own list takes only --hsm-name (_params.py:272), but the vault operation
// exists, so this accepts --vault-name.
func ListPrivateEndpointConnections(ctx context.Context, cmd *cobra.Command, vaultName, resourceGroup string) error {
	client, err := privateEndpointClient()
	if err != nil {
		return err
	}
	pager := client.NewListByResourcePager(resourceGroup, vaultName, nil)
	connections := []*armkeyvault.PrivateEndpointConnection{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list private endpoint connections: %w", err)
		}
		connections = append(connections, page.Value...)
	}
	return output.PrintJSON(cmd, connections)
}

// ListPrivateLinkResources lists the private link resources of a vault
// (custom.py:1894). azure-cli unwraps the "value" property into a plain list.
func ListPrivateLinkResources(ctx context.Context, cmd *cobra.Command, vaultName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}
	client, err := armkeyvault.NewPrivateLinkResourcesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create private link resources client: %w", err)
	}
	resp, err := client.ListByVault(ctx, resourceGroup, vaultName, nil)
	if err != nil {
		return fmt.Errorf("failed to list private link resources: %w", err)
	}
	return output.PrintJSON(cmd, resp.Value)
}
