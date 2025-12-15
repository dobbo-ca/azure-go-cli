package aks

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/internal/network/bastion"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

// BastionOptions contains options for the bastion tunnel
type BastionOptions struct {
	ClusterName          string
	ResourceGroup        string
	BastionResourceID    string
	SubscriptionOverride string
	Admin                bool
	Port                 int
	Command              string // Command to run with KUBECONFIG set (e.g., "k9s" or "kubectl get pods")
	BufferConfig         bastion.BufferConfig
}

// Bastion is a convenience wrapper around network bastion tunnel
// It fetches the AKS cluster details and calls the bastion tunnel with appropriate parameters
func Bastion(ctx context.Context, opts BastionOptions) error {
	// Check dependencies
	missing := CheckDependencies()
	if len(missing) > 0 {
		fmt.Printf("Warning: The following required tools are not installed: %v\n", missing)
		fmt.Println("Please install them using: sudo az aks install-cli")
		fmt.Println("(Note: This command requires sudo to install to /usr/local/bin)")
		fmt.Println()
	}

	// Use random high port if not specified
	port := opts.Port
	if port == 0 {
		rand.Seed(time.Now().UnixNano())
		port = 49152 + rand.Intn(16384) // Ephemeral port range: 49152-65535
		logger.Debug("Using random port: %d", port)
	}

	clusterName := opts.ClusterName
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetSubscription(opts.SubscriptionOverride)
	if err != nil {
		return err
	}

	// Get cluster info
	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create AKS client: %w", err)
	}

	cluster, err := client.Get(ctx, opts.ResourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	if cluster.ID == nil {
		return fmt.Errorf("cluster ID not found")
	}

	// Get cluster FQDN for kubeconfig
	clusterFQDN := ""
	if cluster.Properties != nil && cluster.Properties.Fqdn != nil {
		clusterFQDN = *cluster.Properties.Fqdn
	}

	// Create temporary kubeconfig
	kubeconfigPath, err := CreateTempKubeconfig(ctx, clusterName, clusterFQDN, port)
	if err != nil {
		return fmt.Errorf("failed to create temporary kubeconfig: %w", err)
	}
	defer func() {
		// Clean up temp directory
		tmpDir := filepath.Dir(filepath.Dir(kubeconfigPath))
		logger.Debug("Cleaning up temporary directory: %s", tmpDir)
		os.RemoveAll(tmpDir)
	}()

	fmt.Printf("Merged \"%s\" as current context in %s\n", clusterName, kubeconfigPath)
	fmt.Println("Converted kubeconfig to use Azure CLI authentication.")

	fmt.Printf("Opening tunnel to AKS cluster %s through Bastion...\n", clusterName)

	// Extract bastion details from resource ID
	bastionName, bastionRG, err := parseBastionResourceID(opts.BastionResourceID)
	if err != nil {
		return fmt.Errorf("failed to parse bastion resource ID: %w", err)
	}

	// Start bastion tunnel in background
	tunnelCtx, cancelTunnel := context.WithCancel(ctx)
	defer cancelTunnel()

	tunnelErrCh := make(chan error, 1)
	go func() {
		tunnelErrCh <- bastion.Tunnel(tunnelCtx, bastionName, bastionRG, *cluster.ID, 443, port, opts.BufferConfig)
	}()

	// Wait a moment for tunnel to establish
	time.Sleep(2 * time.Second)

	// Check if tunnel failed to start
	select {
	case err := <-tunnelErrCh:
		if err != nil {
			return fmt.Errorf("tunnel failed to start: %w", err)
		}
	default:
		// Tunnel is running
	}

	// If --cmd flag is set, authenticate and run the specified command
	if opts.Command != "" {
		// Perform authentication first (handles device code flow)
		if err := AuthenticateKubeconfig(ctx, kubeconfigPath); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Now run the specified command
		if err := RunCommand(ctx, kubeconfigPath, opts.Command); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
		// Command exited, clean up
		fmt.Println("\nCommand exited, shutting down tunnel...")
		return nil
	}

	// Otherwise, show export command and copy to clipboard
	exportCmd := fmt.Sprintf("export KUBECONFIG=%s", kubeconfigPath)

	fmt.Printf("\n%s\n", exportCmd)

	// Copy to clipboard
	if err := copyToClipboard(exportCmd); err != nil {
		logger.Debug("Failed to copy to clipboard: %v", err)
	} else {
		fmt.Println("✓ Copied to clipboard")
	}

	fmt.Println("\nPress Ctrl+C to close the tunnel")

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Wait for tunnel to exit, error, or interrupt signal
	select {
	case err := <-tunnelErrCh:
		return err
	case <-sigCh:
		fmt.Println("\nReceived interrupt signal, shutting down tunnel...")
		cancelTunnel()
		return nil
	}
}

func parseBastionResourceID(resourceID string) (name string, resourceGroup string, err error) {
	// Parse Azure resource IDs
	// Supported formats:
	// - /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/bastionHosts/{name}
	// - /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/virtualNetworks/{name}

	if resourceID == "" {
		return "", "", fmt.Errorf("resource ID cannot be empty")
	}

	// Split by '/' and remove empty strings
	parts := make([]string, 0)
	for _, part := range splitResourceID(resourceID) {
		if part != "" {
			parts = append(parts, part)
		}
	}

	// Minimum valid resource ID should have at least:
	// subscriptions, {id}, resourceGroups, {name}, providers, {namespace}, {type}, {name}
	if len(parts) < 8 {
		return "", "", fmt.Errorf("invalid resource ID format: too few segments")
	}

	// Find resource group
	rgIndex := -1
	for i, part := range parts {
		if part == "resourceGroups" && i+1 < len(parts) {
			rgIndex = i + 1
			break
		}
	}
	if rgIndex == -1 {
		return "", "", fmt.Errorf("resource group not found in resource ID")
	}
	resourceGroup = parts[rgIndex]

	// Find provider and resource type
	providerIndex := -1
	for i, part := range parts {
		if part == "providers" && i+1 < len(parts) {
			providerIndex = i + 1
			break
		}
	}
	if providerIndex == -1 {
		return "", "", fmt.Errorf("provider not found in resource ID")
	}

	// Validate it's a Network resource
	if parts[providerIndex] != "Microsoft.Network" {
		return "", "", fmt.Errorf("expected Microsoft.Network provider, got: %s", parts[providerIndex])
	}

	// Check resource type and get name
	if providerIndex+2 >= len(parts) {
		return "", "", fmt.Errorf("invalid resource ID: missing resource type or name")
	}

	resourceType := parts[providerIndex+1]
	name = parts[providerIndex+2]

	// Support both bastionHosts and virtualNetworks (where the vnet name might be the bastion name)
	switch resourceType {
	case "bastionHosts":
		// Direct bastion host reference
		return name, resourceGroup, nil
	case "virtualNetworks":
		// VNet reference - assume the vnet name is the bastion name
		// This is a common pattern where bastions are named after their containing vnet
		return name, resourceGroup, nil
	default:
		return "", "", fmt.Errorf("unsupported resource type: %s (expected bastionHosts or virtualNetworks)", resourceType)
	}
}

func splitResourceID(resourceID string) []string {
	result := make([]string, 0)
	current := ""
	for _, char := range resourceID {
		if char == '/' {
			result = append(result, current)
			current = ""
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
