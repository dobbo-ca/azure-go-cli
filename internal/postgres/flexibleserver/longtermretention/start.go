package longtermretention

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Start(ctx context.Context, cmd *cobra.Command, resourceGroup, serverName, backupName, sasURL string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewFlexibleServerClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create flexible server client: %w", err)
	}

	parameters := armpostgresqlflexibleservers.LtrBackupRequest{
		BackupSettings: &armpostgresqlflexibleservers.BackupSettings{
			BackupName: to.Ptr(backupName),
		},
		TargetDetails: &armpostgresqlflexibleservers.BackupStoreDetails{
			SasURIList: []*string{to.Ptr(sasURL)},
		},
	}

	fmt.Printf("Starting long-term retention backup '%s' for server '%s'...\n", backupName, serverName)

	poller, err := client.BeginStartLtrBackup(ctx, resourceGroup, serverName, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin LTR backup: %w", err)
	}

	if noWait {
		return output.PrintJSON(cmd, map[string]string{"status": "LTR backup started."})
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("LTR backup failed: %w", err)
	}

	return output.PrintJSON(cmd, resp.LtrBackupResponse)
}
