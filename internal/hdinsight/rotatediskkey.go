package hdinsight

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func RotateDiskEncryptionKey(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName, vaultURI, keyName, keyVersion string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armhdinsight.NewClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create clusters client: %w", err)
	}

	params := armhdinsight.ClusterDiskEncryptionParameters{
		VaultURI:   to.Ptr(vaultURI),
		KeyName:    to.Ptr(keyName),
		KeyVersion: to.Ptr(keyVersion),
	}

	fmt.Printf("Rotating disk encryption key for cluster '%s'...\n", clusterName)
	poller, err := client.BeginRotateDiskEncryptionKey(ctx, resourceGroup, clusterName, params, nil)
	if err != nil {
		return fmt.Errorf("failed to begin key rotation: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "disk encryption key rotation started"})
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("key rotation failed: %w", err)
	}

	return output.PrintJSON(cmd, map[string]string{"status": fmt.Sprintf("disk encryption key rotated for '%s'.", clusterName)})
}
