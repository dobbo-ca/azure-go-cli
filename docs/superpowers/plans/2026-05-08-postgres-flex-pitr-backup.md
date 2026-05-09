# Postgres Flexible Server PITR + Backup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `az postgres flexible-server backup` (create/list/show/delete) and `az postgres flexible-server restore` + `geo-restore` to expose the built-in Azure Postgres flexible-server PITR + on-demand backup APIs.

**Architecture:** Wraps Azure SDK `armpostgresqlflexibleservers/v4`. Backup CRUD uses `BackupsClient`. PITR/Geo-restore reuses `ServersClient.BeginCreate` with `CreateMode=PointInTimeRestore` / `GeoRestore` and the source server's resource ID. New `backup` subcommand group nested under `flexible-server`. `restore` and `geo-restore` are siblings of `create`.

**Tech Stack:** Go, cobra, `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4`, existing `pkg/azure`, `pkg/config`, `pkg/output` helpers.

**Verification approach:** This codebase does not unit-test SDK-call command files (only parser logic in `aks/kubeconfig_test.go` and `resource/resolve_test.go`). Each task verifies by `make build` succeeding and `./bin/az/az ... --help` rendering correctly. End-to-end smoke tests against a real subscription are out of scope for the plan but must be done manually before merge.

---

### Task 1: Scaffold backup subcommand group

**Files:**
- Create: `internal/postgres/flexibleserver/backup/commands.go`

- [ ] **Step 1: Create commands.go with empty group**

```go
package backup

import (
	"github.com/spf13/cobra"
)

func NewBackupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage on-demand backups for a PostgreSQL flexible server",
		Long:  "Create, list, show, and delete on-demand backups for an Azure Database for PostgreSQL flexible server. Automated PITR backups are managed by the service and exposed via list/show.",
	}
	return cmd
}
```

- [ ] **Step 2: Verify build**

Run: `make build`
Expected: success, `bin/az/az` binary present.

- [ ] **Step 3: Commit**

```bash
git add internal/postgres/flexibleserver/backup/commands.go
git commit -m "feat(postgres): scaffold flexible-server backup subcommand group"
```

---

### Task 2: Implement `backup list`

**Files:**
- Create: `internal/postgres/flexibleserver/backup/list.go`
- Modify: `internal/postgres/flexibleserver/backup/commands.go`

- [ ] **Step 1: Write list.go**

```go
package backup

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func List(ctx context.Context, resourceGroup, serverName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewBackupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create backups client: %w", err)
	}

	var backups []*armpostgresqlflexibleservers.ServerBackup
	pager := client.NewListByServerPager(resourceGroup, serverName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list backups: %w", err)
		}
		backups = append(backups, page.Value...)
	}

	data, err := json.MarshalIndent(backups, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format backups: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
```

- [ ] **Step 2: Wire list subcommand in commands.go**

Replace `commands.go` with:

```go
package backup

import (
	"context"

	"github.com/spf13/cobra"
)

func NewBackupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage on-demand backups for a PostgreSQL flexible server",
		Long:  "Create, list, show, and delete on-demand backups for an Azure Database for PostgreSQL flexible server. Automated PITR backups are managed by the service and exposed via list/show.",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List backups for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			return List(context.Background(), rg, server)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.Flags().String("server-name", "", "Flexible server name")
	listCmd.MarkFlagRequired("resource-group")
	listCmd.MarkFlagRequired("server-name")

	cmd.AddCommand(listCmd)
	return cmd
}
```

- [ ] **Step 3: Verify build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/postgres/flexibleserver/backup/list.go internal/postgres/flexibleserver/backup/commands.go
git commit -m "feat(postgres): list flexible-server backups"
```

---

### Task 3: Implement `backup show`

**Files:**
- Create: `internal/postgres/flexibleserver/backup/show.go`
- Modify: `internal/postgres/flexibleserver/backup/commands.go`

- [ ] **Step 1: Write show.go**

```go
package backup

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Show(ctx context.Context, resourceGroup, serverName, backupName string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewBackupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create backups client: %w", err)
	}

	resp, err := client.Get(ctx, resourceGroup, serverName, backupName, nil)
	if err != nil {
		return fmt.Errorf("failed to get backup: %w", err)
	}

	data, err := json.MarshalIndent(resp.ServerBackup, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format backup: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
```

- [ ] **Step 2: Add show subcommand in commands.go**

Insert before `cmd.AddCommand(listCmd)`:

```go
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a backup for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("name")
			return Show(context.Background(), rg, server, name)
		},
	}
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.Flags().String("server-name", "", "Flexible server name")
	showCmd.Flags().StringP("name", "n", "", "Backup name")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("server-name")
	showCmd.MarkFlagRequired("name")
```

Then update `cmd.AddCommand(listCmd)` to `cmd.AddCommand(listCmd, showCmd)`.

- [ ] **Step 3: Verify build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/postgres/flexibleserver/backup/show.go internal/postgres/flexibleserver/backup/commands.go
git commit -m "feat(postgres): show flexible-server backup"
```

---

### Task 4: Implement `backup create`

**Files:**
- Create: `internal/postgres/flexibleserver/backup/create.go`
- Modify: `internal/postgres/flexibleserver/backup/commands.go`

- [ ] **Step 1: Write create.go**

The Azure REST contract: PUT `/backups/{backupName}` triggers an on-demand backup with the given name. No body is required; the SDK exposes only the four positional args (resource group, server, backup name, options).

```go
package backup

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Create(ctx context.Context, resourceGroup, serverName, backupName string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewBackupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create backups client: %w", err)
	}

	fmt.Printf("Triggering on-demand backup '%s' on server '%s'...\n", backupName, serverName)
	poller, err := client.BeginCreate(ctx, resourceGroup, serverName, backupName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin backup: %w", err)
	}

	if noWait {
		fmt.Println(`{"status": "On-demand backup started. Use 'az postgres flexible-server backup show' to monitor."}`)
		return nil
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("backup operation failed: %w", err)
	}

	data, err := json.MarshalIndent(resp.ServerBackup, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format backup: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
```

- [ ] **Step 2: Add create subcommand in commands.go**

Insert before the final `cmd.AddCommand(...)`:

```go
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Trigger an on-demand backup for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Create(context.Background(), rg, server, name, noWait)
		},
	}
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().String("server-name", "", "Flexible server name")
	createCmd.Flags().StringP("name", "n", "", "Backup name")
	createCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("server-name")
	createCmd.MarkFlagRequired("name")
```

Update `cmd.AddCommand(listCmd, showCmd)` to `cmd.AddCommand(listCmd, showCmd, createCmd)`.

- [ ] **Step 3: Verify build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/postgres/flexibleserver/backup/create.go internal/postgres/flexibleserver/backup/commands.go
git commit -m "feat(postgres): create on-demand flexible-server backup"
```

---

### Task 5: Implement `backup delete`

**Files:**
- Create: `internal/postgres/flexibleserver/backup/delete.go`
- Modify: `internal/postgres/flexibleserver/backup/commands.go`

- [ ] **Step 1: Write delete.go**

```go
package backup

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, resourceGroup, serverName, backupName string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armpostgresqlflexibleservers.NewBackupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create backups client: %w", err)
	}

	poller, err := client.BeginDelete(ctx, resourceGroup, serverName, backupName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin backup delete: %w", err)
	}

	if noWait {
		fmt.Println(`{"status": "Backup delete started."}`)
		return nil
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("backup delete failed: %w", err)
	}
	fmt.Printf(`{"status": "Backup '%s' deleted."}`+"\n", backupName)
	return nil
}
```

- [ ] **Step 2: Add delete subcommand in commands.go**

Insert before the final `cmd.AddCommand(...)`:

```go
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an on-demand backup for a PostgreSQL flexible server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rg, _ := cmd.Flags().GetString("resource-group")
			server, _ := cmd.Flags().GetString("server-name")
			name, _ := cmd.Flags().GetString("name")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), rg, server, name, noWait)
		},
	}
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().String("server-name", "", "Flexible server name")
	deleteCmd.Flags().StringP("name", "n", "", "Backup name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	deleteCmd.MarkFlagRequired("resource-group")
	deleteCmd.MarkFlagRequired("server-name")
	deleteCmd.MarkFlagRequired("name")
```

Update `cmd.AddCommand(...)` line to `cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)`.

- [ ] **Step 3: Verify build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/postgres/flexibleserver/backup/delete.go internal/postgres/flexibleserver/backup/commands.go
git commit -m "feat(postgres): delete flexible-server backup"
```

---

### Task 6: Implement `flexible-server restore` (PITR)

**Files:**
- Create: `internal/postgres/flexibleserver/restore.go`
- Modify: `internal/postgres/flexibleserver/commands.go`

PITR creates a *new* server from a source server's continuous backup at a specified UTC time.

- [ ] **Step 1: Write restore.go**

```go
package flexibleserver

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// Restore creates a new flexible server from a point-in-time restore of an existing source server.
// restoreTime must be RFC3339 (e.g. 2026-05-08T14:30:00Z).
func Restore(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, sourceServerID, restoreTime string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	t, err := time.Parse(time.RFC3339, restoreTime)
	if err != nil {
		return fmt.Errorf("invalid --restore-time %q: must be RFC3339 (e.g. 2026-05-08T14:30:00Z): %w", restoreTime, err)
	}

	client, err := armpostgresqlflexibleservers.NewServersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create servers client: %w", err)
	}

	parameters := armpostgresqlflexibleservers.Server{
		Location: to.Ptr(location),
		Properties: &armpostgresqlflexibleservers.ServerProperties{
			CreateMode:             to.Ptr(armpostgresqlflexibleservers.CreateModePointInTimeRestore),
			SourceServerResourceID: to.Ptr(sourceServerID),
			PointInTimeUTC:         to.Ptr(t),
		},
	}

	fmt.Printf("Restoring '%s' to point-in-time %s from %s...\n", name, t.Format(time.RFC3339), sourceServerID)
	poller, err := client.BeginCreate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin PITR restore: %w", err)
	}

	if noWait {
		fmt.Println(`{"status": "PITR restore started."}`)
		return nil
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("PITR restore failed: %w", err)
	}
	return output.PrintJSON(cmd, result.Server)
}
```

- [ ] **Step 2: Wire restore subcommand in commands.go**

Insert before the final `cmd.AddCommand(...)` line in `internal/postgres/flexibleserver/commands.go`:

```go
	restoreCmd := &cobra.Command{
		Use:   "restore",
		Short: "Point-in-time restore a PostgreSQL flexible server to a new server",
		Long:  "Creates a new PostgreSQL flexible server by performing a point-in-time restore from an existing source server. The source server must be running and within the configured backup retention window.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			sourceID, _ := cmd.Flags().GetString("source-server")
			restoreTime, _ := cmd.Flags().GetString("restore-time")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Restore(context.Background(), cmd, name, rg, location, sourceID, restoreTime, noWait)
		},
	}
	restoreCmd.Flags().StringP("name", "n", "", "Name of the new restored server")
	restoreCmd.Flags().StringP("resource-group", "g", "", "Resource group for the new server")
	restoreCmd.Flags().StringP("location", "l", "", "Location of the new server (must match source for PITR)")
	restoreCmd.Flags().String("source-server", "", "Full Azure resource ID of the source flexible server")
	restoreCmd.Flags().String("restore-time", "", "Point-in-time UTC, RFC3339 (e.g. 2026-05-08T14:30:00Z)")
	restoreCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	restoreCmd.MarkFlagRequired("name")
	restoreCmd.MarkFlagRequired("resource-group")
	restoreCmd.MarkFlagRequired("location")
	restoreCmd.MarkFlagRequired("source-server")
	restoreCmd.MarkFlagRequired("restore-time")
```

- [ ] **Step 3: Verify build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/postgres/flexibleserver/restore.go internal/postgres/flexibleserver/commands.go
git commit -m "feat(postgres): point-in-time restore for flexible servers"
```

---

### Task 7: Implement `flexible-server geo-restore`

**Files:**
- Create: `internal/postgres/flexibleserver/georestore.go`
- Modify: `internal/postgres/flexibleserver/commands.go`

Geo-restore is identical to PITR except `CreateMode=GeoRestore` and the new server's location is the paired (geo-redundant) region. Source server must have geo-redundant backup enabled.

- [ ] **Step 1: Write georestore.go**

```go
package flexibleserver

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// GeoRestore creates a new flexible server in a geo-paired region from the source server's geo-redundant backup.
func GeoRestore(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, sourceServerID, restoreTime string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	props := &armpostgresqlflexibleservers.ServerProperties{
		CreateMode:             to.Ptr(armpostgresqlflexibleservers.CreateModeGeoRestore),
		SourceServerResourceID: to.Ptr(sourceServerID),
	}

	if restoreTime != "" {
		t, err := time.Parse(time.RFC3339, restoreTime)
		if err != nil {
			return fmt.Errorf("invalid --restore-time %q: must be RFC3339: %w", restoreTime, err)
		}
		props.PointInTimeUTC = to.Ptr(t)
	}

	client, err := armpostgresqlflexibleservers.NewServersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create servers client: %w", err)
	}

	parameters := armpostgresqlflexibleservers.Server{
		Location:   to.Ptr(location),
		Properties: props,
	}

	fmt.Printf("Geo-restoring '%s' in %s from %s...\n", name, location, sourceServerID)
	poller, err := client.BeginCreate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin geo-restore: %w", err)
	}

	if noWait {
		fmt.Println(`{"status": "Geo-restore started."}`)
		return nil
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("geo-restore failed: %w", err)
	}
	return output.PrintJSON(cmd, result.Server)
}
```

- [ ] **Step 2: Wire geo-restore subcommand in commands.go**

Insert before the final `cmd.AddCommand(...)` line:

```go
	geoRestoreCmd := &cobra.Command{
		Use:   "geo-restore",
		Short: "Geo-restore a PostgreSQL flexible server to a paired region",
		Long:  "Creates a new PostgreSQL flexible server in a paired region from the source server's geo-redundant backup. Source server must have geo-redundant backup enabled.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			rg, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			sourceID, _ := cmd.Flags().GetString("source-server")
			restoreTime, _ := cmd.Flags().GetString("restore-time")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return GeoRestore(context.Background(), cmd, name, rg, location, sourceID, restoreTime, noWait)
		},
	}
	geoRestoreCmd.Flags().StringP("name", "n", "", "Name of the new restored server")
	geoRestoreCmd.Flags().StringP("resource-group", "g", "", "Resource group for the new server")
	geoRestoreCmd.Flags().StringP("location", "l", "", "Target location (paired region)")
	geoRestoreCmd.Flags().String("source-server", "", "Full Azure resource ID of the source flexible server")
	geoRestoreCmd.Flags().String("restore-time", "", "Optional point-in-time UTC RFC3339; defaults to latest available geo backup")
	geoRestoreCmd.Flags().Bool("no-wait", false, "Do not wait for the operation to complete")
	geoRestoreCmd.MarkFlagRequired("name")
	geoRestoreCmd.MarkFlagRequired("resource-group")
	geoRestoreCmd.MarkFlagRequired("location")
	geoRestoreCmd.MarkFlagRequired("source-server")
```

- [ ] **Step 3: Verify build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/postgres/flexibleserver/georestore.go internal/postgres/flexibleserver/commands.go
git commit -m "feat(postgres): geo-restore for flexible servers"
```

---

### Task 8: Register `backup` group + restore commands on flexible-server parent

**Files:**
- Modify: `internal/postgres/flexibleserver/commands.go`

- [ ] **Step 1: Import backup package and add it to AddCommand**

Add import:

```go
"github.com/cdobbyn/azure-go-cli/internal/postgres/flexibleserver/backup"
```

Update final `cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, listSkusCmd)` to include the new subcommands:

```go
cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, listSkusCmd, restoreCmd, geoRestoreCmd, backup.NewBackupCommand())
```

- [ ] **Step 2: Verify build**

Run: `make build`
Expected: success.

- [ ] **Step 3: Verify help output renders all new subcommands**

Run: `./bin/az/az postgres flexible-server --help`
Expected: output contains `backup`, `restore`, `geo-restore`.

Run: `./bin/az/az postgres flexible-server backup --help`
Expected: output contains `create`, `list`, `show`, `delete`.

Run: `./bin/az/az postgres flexible-server restore --help`
Expected: shows all required flags `--name`, `--resource-group`, `--location`, `--source-server`, `--restore-time`.

Run: `./bin/az/az postgres flexible-server geo-restore --help`
Expected: shows all required flags except `--restore-time` (optional).

- [ ] **Step 4: Commit**

```bash
git add internal/postgres/flexibleserver/commands.go
git commit -m "feat(postgres): register flexible-server backup, restore, geo-restore commands"
```

---

### Task 9: Add `--backup-retention` and `--geo-redundant-backup` to `flexible-server create`

PITR depth is configured at server-create time via `Backup.BackupRetentionDays` (7-35) and `Backup.GeoRedundantBackup`. Today `create.go` hardcodes 7 days / disabled. Expose them so users can opt into longer retention and geo-redundant backups (prerequisite for `geo-restore`).

**Files:**
- Modify: `internal/postgres/flexibleserver/create.go`
- Modify: `internal/postgres/flexibleserver/commands.go`

- [ ] **Step 1: Extend `Create` signature in create.go**

Replace the function signature and body relevant section. New signature:

```go
func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location, adminUser, adminPassword, version, tier, sku string, storageSizeGB, backupRetentionDays int32, geoRedundantBackup bool, tags map[string]string) error {
```

Replace the `Backup` struct literal with:

```go
		geoRedundancy := armpostgresqlflexibleservers.GeoRedundantBackupEnumDisabled
		if geoRedundantBackup {
			geoRedundancy = armpostgresqlflexibleservers.GeoRedundantBackupEnumEnabled
		}
```

(insert before `parameters := armpostgresqlflexibleservers.Server{...}`)

Then in `parameters.Properties.Backup`:

```go
				Backup: &armpostgresqlflexibleservers.Backup{
					BackupRetentionDays: to.Ptr(backupRetentionDays),
					GeoRedundantBackup:  to.Ptr(geoRedundancy),
				},
```

- [ ] **Step 2: Add flags + plumb in commands.go**

Add flags to `createCmd`:

```go
createCmd.Flags().Int32("backup-retention", 7, "Backup retention in days (7-35)")
createCmd.Flags().Bool("geo-redundant-backup", false, "Enable geo-redundant backup (required for geo-restore)")
```

Update the `RunE` callback:

```go
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			adminUser, _ := cmd.Flags().GetString("admin-user")
			adminPassword, _ := cmd.Flags().GetString("admin-password")
			version, _ := cmd.Flags().GetString("version")
			tier, _ := cmd.Flags().GetString("tier")
			sku, _ := cmd.Flags().GetString("sku-name")
			storageSizeGB, _ := cmd.Flags().GetInt32("storage-size")
			backupRetention, _ := cmd.Flags().GetInt32("backup-retention")
			geoRedundant, _ := cmd.Flags().GetBool("geo-redundant-backup")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, resourceGroup, location, adminUser, adminPassword, version, tier, sku, storageSizeGB, backupRetention, geoRedundant, tags)
		},
```

- [ ] **Step 3: Verify build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Verify help output**

Run: `./bin/az/az postgres flexible-server create --help`
Expected: shows `--backup-retention` and `--geo-redundant-backup` flags.

- [ ] **Step 5: Commit**

```bash
git add internal/postgres/flexibleserver/create.go internal/postgres/flexibleserver/commands.go
git commit -m "feat(postgres): expose backup retention + geo-redundant backup on create"
```

---

### Task 10: End-to-end smoke test (manual)

Verifies wiring against a real subscription. Skip in CI; do before merging.

- [ ] **Step 1: Pick a non-prod test resource group + flexible server name**

```bash
RG=<rg>
SRC=<existing-flex-server>
```

- [ ] **Step 2: Trigger an on-demand backup**

```bash
./bin/az/az postgres flexible-server backup create -g $RG --server-name $SRC -n manual-$(date +%s)
```

Expected: JSON describing the new backup with `properties.backupType` and `completedTime`.

- [ ] **Step 3: List backups**

```bash
./bin/az/az postgres flexible-server backup list -g $RG --server-name $SRC
```

Expected: JSON array including the backup just created plus automated backups.

- [ ] **Step 4: PITR to a new server**

```bash
NEW=$SRC-pitr-$(date +%s)
SRC_ID=$(./bin/az/az postgres flexible-server show -g $RG -n $SRC | jq -r .ID)
./bin/az/az postgres flexible-server restore \
  -g $RG -n $NEW -l <same-region-as-src> \
  --source-server "$SRC_ID" \
  --restore-time "$(date -u -v-15M +%Y-%m-%dT%H:%M:%SZ)" \
  --no-wait
```

Expected: `{"status": "PITR restore started."}` — then verify in portal or via `az postgres flexible-server show`.

- [ ] **Step 5: Cleanup test resources**

```bash
./bin/az/az postgres flexible-server delete -g $RG -n $NEW --no-wait
```

- [ ] **Step 6: No commit — this task is verification only.**

---

## Open follow-ups (not blocking tonight)

These are intentionally out of scope but worth tracking:

- `flexible-server revive-dropped` (CreateMode=ReviveDropped) — restore a recently deleted server.
- `flexible-server replica` commands (CreateMode=Replica) for read replicas.
- Long-term retention (LTR) backup commands (`LtrServerBackupOperation` types) — separate workstream.
- Update `--backup-retention` / `--geo-redundant-backup` post-create via `ServersClient.BeginUpdate` (mirror Azure CLI's `update` verb).

---

## Self-review notes

- Spec coverage: backup CRUD, PITR, geo-restore, plus backup-config exposure on create — covered by Tasks 1-9. Smoke test in Task 10.
- Naming consistent: `--server-name` used uniformly across backup subcommands; `--source-server` used for restore/geo-restore.
- No placeholders; every code block is complete.
- Type consistency: `armpostgresqlflexibleservers.NewBackupsClient` and `NewServersClient` are the only two SDK clients; `CreateModePointInTimeRestore` and `CreateModeGeoRestore` confirmed in v4.1.0 constants. `GeoRedundantBackupEnumEnabled` / `Disabled` confirmed in `create.go`'s existing usage.
