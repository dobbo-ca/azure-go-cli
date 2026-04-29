# Data Protection CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `az dataprotection` command group mirroring the official Azure CLI, covering backup vaults, policies, instances, recovery points, and jobs — with restore trigger as the highest priority.

**Architecture:** New `internal/dataprotection/` package tree with subpackages per command group (backupvault, backuppolicy, backupinstance, recoverypoint, job). Each subpackage follows the existing cobra pattern: `commands.go` for command definitions + separate files per operation. Uses the `armdataprotection/v3` Go SDK.

**Tech Stack:** Go, Cobra CLI, `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3` v3.1.0

**JIRA:** TEC-2915

---

## File Structure

```
internal/dataprotection/
├── commands.go                          # Root "dataprotection" command
├── backupvault/
│   ├── commands.go                      # backup-vault subcommands
│   ├── create.go                        # BeginCreateOrUpdate
│   ├── show.go                          # Get
│   ├── list.go                          # NewGetInResourceGroupPager / NewGetInSubscriptionPager
│   ├── update.go                        # BeginUpdate
│   └── delete.go                        # BeginDelete
├── backuppolicy/
│   ├── commands.go                      # backup-policy subcommands
│   ├── create.go                        # CreateOrUpdate
│   ├── show.go                          # Get
│   ├── list.go                          # NewListPager
│   ├── delete.go                        # Delete
│   └── defaultpolicy.go                # get-default-policy-template (client-side)
├── backupinstance/
│   ├── commands.go                      # backup-instance subcommands
│   ├── create.go                        # BeginCreateOrUpdate
│   ├── show.go                          # Get
│   ├── list.go                          # NewListPager
│   ├── delete.go                        # BeginDelete
│   ├── adhocbackup.go                   # BeginAdhocBackup
│   ├── validateforbackup.go             # BeginValidateForBackup
│   ├── validateforrestore.go            # BeginValidateForRestore
│   ├── restore.go                       # BeginTriggerRestore
│   ├── stopprotection.go               # BeginStopProtection
│   ├── suspendbackup.go                # BeginSuspendBackups
│   └── resumeprotection.go             # BeginResumeProtection
├── recoverypoint/
│   ├── commands.go                      # recovery-point subcommands
│   ├── list.go                          # NewListPager
│   └── show.go                          # Get
└── job/
    ├── commands.go                      # job subcommands
    ├── list.go                          # NewListPager
    └── show.go                          # Get
```

**Modified files:**
- `cmd/az/main.go` — register `dataprotection.NewDataProtectionCommand()`
- `go.mod` / `go.sum` — add `armdataprotection/v3` dependency

---

## Priority Order

Tasks are ordered with restore trigger first (the most critical need), then the supporting infrastructure around it, expanding outward.

---

### Task 1: Add SDK Dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add the armdataprotection/v3 SDK dependency**

```bash
cd /Users/christopherdobbyn/work/dobbo-ca/azure-go-cli && go get github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3@v3.1.0
```

- [ ] **Step 2: Verify dependency was added**

```bash
grep dataprotection go.mod
```

Expected: line containing `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3 v3.1.0`

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add armdataprotection/v3 SDK dependency for TEC-2915"
```

---

### Task 2: Root Command + Registration

**Files:**
- Create: `internal/dataprotection/commands.go`
- Modify: `cmd/az/main.go`

- [ ] **Step 1: Create the root dataprotection command**

Create `internal/dataprotection/commands.go`:

```go
package dataprotection

import (
  "github.com/spf13/cobra"
)

func NewDataProtectionCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "dataprotection",
    Short: "Manage Azure Data Protection",
    Long:  "Commands to manage Azure Data Protection backup vaults, policies, instances, and restore operations",
  }

  return cmd
}
```

- [ ] **Step 2: Register in main.go**

In `cmd/az/main.go`, add import `"github.com/cdobbyn/azure-go-cli/internal/dataprotection"` and add `dataprotection.NewDataProtectionCommand()` to the `rootCmd.AddCommand(...)` call.

- [ ] **Step 3: Build and verify**

```bash
make build && ./bin/az/az dataprotection --help
```

Expected: shows "Manage Azure Data Protection" help text.

- [ ] **Step 4: Commit**

```bash
git add internal/dataprotection/commands.go cmd/az/main.go
git commit -m "feat: add dataprotection root command (TEC-2915)"
```

---

### Task 3: Backup Instance — Restore Trigger (Critical Path)

This is the highest priority operation. Implements `az dataprotection backup-instance restore trigger`.

**Files:**
- Create: `internal/dataprotection/backupinstance/commands.go`
- Create: `internal/dataprotection/backupinstance/restore.go`
- Modify: `internal/dataprotection/commands.go`

- [ ] **Step 1: Create backup-instance commands.go with restore subcommand group**

Create `internal/dataprotection/backupinstance/commands.go`:

```go
package backupinstance

import (
  "github.com/spf13/cobra"
)

func NewBackupInstanceCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "backup-instance",
    Short: "Manage backup instances",
    Long:  "Commands to manage backup instances within a backup vault",
  }

  restoreCmd := newRestoreCommand()

  cmd.AddCommand(restoreCmd)
  return cmd
}

func newRestoreCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "restore",
    Short: "Restore backed up instances",
    Long:  "Commands to restore backed up instances from recovery points",
  }

  cmd.AddCommand(newRestoreTriggerCommand())
  return cmd
}
```

- [ ] **Step 2: Implement restore trigger**

Create `internal/dataprotection/backupinstance/restore.go`:

```go
package backupinstance

import (
  "context"
  "encoding/json"
  "fmt"
  "os"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newRestoreTriggerCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "trigger",
    Short: "Trigger restore of a backup instance",
    Long:  "Triggers a restore operation for a backup instance using a restore request object",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      backupInstanceName, _ := cmd.Flags().GetString("backup-instance-name")
      restoreRequestFile, _ := cmd.Flags().GetString("restore-request-object")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return TriggerRestore(context.Background(), resourceGroup, vaultName, backupInstanceName, restoreRequestFile, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("backup-instance-name", "", "Name of the backup instance")
  cmd.Flags().String("restore-request-object", "", "Path to JSON file containing restore request object")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("backup-instance-name")
  cmd.MarkFlagRequired("restore-request-object")
  return cmd
}

func TriggerRestore(ctx context.Context, resourceGroup, vaultName, backupInstanceName, restoreRequestFile string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  // Read restore request from file
  data, err := os.ReadFile(restoreRequestFile)
  if err != nil {
    return fmt.Errorf("failed to read restore request file %s: %w", restoreRequestFile, err)
  }

  // Parse restore request - determine the type from the JSON
  var raw map[string]interface{}
  if err := json.Unmarshal(data, &raw); err != nil {
    return fmt.Errorf("failed to parse restore request JSON: %w", err)
  }

  objectType, _ := raw["objectType"].(string)

  var restoreRequest armdataprotection.AzureBackupRestoreRequestClassification
  switch objectType {
  case "AzureBackupRecoveryPointBasedRestoreRequest":
    var req armdataprotection.AzureBackupRecoveryPointBasedRestoreRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse recovery point based restore request: %w", err)
    }
    restoreRequest = &req
  case "AzureBackupRecoveryTimeBasedRestoreRequest":
    var req armdataprotection.AzureBackupRecoveryTimeBasedRestoreRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse recovery time based restore request: %w", err)
    }
    restoreRequest = &req
  case "AzureBackupRestoreWithRehydrationRequest":
    var req armdataprotection.AzureBackupRestoreWithRehydrationRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse restore with rehydration request: %w", err)
    }
    restoreRequest = &req
  default:
    // Fall back to recovery-point-based (most common for PG Flex)
    var req armdataprotection.AzureBackupRecoveryPointBasedRestoreRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse restore request (defaulting to recovery-point-based): %w", err)
    }
    restoreRequest = &req
  }

  poller, err := client.BeginTriggerRestore(ctx, resourceGroup, vaultName, backupInstanceName, restoreRequest, nil)
  if err != nil {
    return fmt.Errorf("failed to trigger restore: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Restore operation started. Use 'az dataprotection job list' to monitor progress."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("restore operation failed: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format restore result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 3: Wire backup-instance into root dataprotection command**

Update `internal/dataprotection/commands.go` to import `backupinstance` and add `cmd.AddCommand(backupinstance.NewBackupInstanceCommand())`.

- [ ] **Step 4: Build and verify**

```bash
make build && ./bin/az/az dataprotection backup-instance restore trigger --help
```

Expected: shows help with `--resource-group`, `--vault-name`, `--backup-instance-name`, `--restore-request-object` flags.

- [ ] **Step 5: Commit**

```bash
git add internal/dataprotection/
git commit -m "feat: add dataprotection backup-instance restore trigger (TEC-2915)

Implements the highest priority command for initiating restores
from backup recovery points. Supports recovery-point-based,
time-based, and rehydration restore request types."
```

---

### Task 4: Backup Instance — Validate for Restore

**Files:**
- Create: `internal/dataprotection/backupinstance/validateforrestore.go`
- Modify: `internal/dataprotection/backupinstance/commands.go`

- [ ] **Step 1: Implement validate-for-restore**

Create `internal/dataprotection/backupinstance/validateforrestore.go`:

```go
package backupinstance

import (
  "context"
  "encoding/json"
  "fmt"
  "os"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newValidateForRestoreCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "validate-for-restore",
    Short: "Validate a restore request for a backup instance",
    Long:  "Validates whether a restore can be triggered for a backup instance with the given restore request",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      backupInstanceName, _ := cmd.Flags().GetString("backup-instance-name")
      restoreRequestFile, _ := cmd.Flags().GetString("restore-request-object")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return ValidateForRestore(context.Background(), resourceGroup, vaultName, backupInstanceName, restoreRequestFile, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("backup-instance-name", "", "Name of the backup instance")
  cmd.Flags().String("restore-request-object", "", "Path to JSON file containing restore request object")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("backup-instance-name")
  cmd.MarkFlagRequired("restore-request-object")
  return cmd
}

func ValidateForRestore(ctx context.Context, resourceGroup, vaultName, backupInstanceName, restoreRequestFile string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  data, err := os.ReadFile(restoreRequestFile)
  if err != nil {
    return fmt.Errorf("failed to read restore request file %s: %w", restoreRequestFile, err)
  }

  var raw map[string]interface{}
  if err := json.Unmarshal(data, &raw); err != nil {
    return fmt.Errorf("failed to parse restore request JSON: %w", err)
  }

  objectType, _ := raw["objectType"].(string)

  var restoreRequest armdataprotection.AzureBackupRestoreRequestClassification
  switch objectType {
  case "AzureBackupRecoveryPointBasedRestoreRequest":
    var req armdataprotection.AzureBackupRecoveryPointBasedRestoreRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse restore request: %w", err)
    }
    restoreRequest = &req
  case "AzureBackupRecoveryTimeBasedRestoreRequest":
    var req armdataprotection.AzureBackupRecoveryTimeBasedRestoreRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse restore request: %w", err)
    }
    restoreRequest = &req
  case "AzureBackupRestoreWithRehydrationRequest":
    var req armdataprotection.AzureBackupRestoreWithRehydrationRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse restore request: %w", err)
    }
    restoreRequest = &req
  default:
    var req armdataprotection.AzureBackupRecoveryPointBasedRestoreRequest
    if err := json.Unmarshal(data, &req); err != nil {
      return fmt.Errorf("failed to parse restore request: %w", err)
    }
    restoreRequest = &req
  }

  validateRequest := armdataprotection.ValidateRestoreRequestObject{
    RestoreRequestObject: restoreRequest,
  }

  poller, err := client.BeginValidateForRestore(ctx, resourceGroup, vaultName, backupInstanceName, validateRequest, nil)
  if err != nil {
    return fmt.Errorf("failed to validate for restore: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Validation started."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("restore validation failed: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format validation result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 2: Register in commands.go**

Add `cmd.AddCommand(newValidateForRestoreCommand())` in `NewBackupInstanceCommand()`.

- [ ] **Step 3: Build and verify**

```bash
make build && ./bin/az/az dataprotection backup-instance validate-for-restore --help
```

- [ ] **Step 4: Commit**

```bash
git add internal/dataprotection/backupinstance/validateforrestore.go internal/dataprotection/backupinstance/commands.go
git commit -m "feat: add dataprotection backup-instance validate-for-restore (TEC-2915)"
```

---

### Task 5: Backup Instance — Adhoc Backup

**Files:**
- Create: `internal/dataprotection/backupinstance/adhocbackup.go`
- Modify: `internal/dataprotection/backupinstance/commands.go`

- [ ] **Step 1: Implement adhoc-backup**

Create `internal/dataprotection/backupinstance/adhocbackup.go`:

```go
package backupinstance

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newAdhocBackupCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "adhoc-backup",
    Short: "Trigger an on-demand backup",
    Long:  "Triggers an on-demand backup for a backup instance",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      backupInstanceName, _ := cmd.Flags().GetString("name")
      ruleName, _ := cmd.Flags().GetString("rule-name")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return AdhocBackup(context.Background(), cmd, resourceGroup, vaultName, backupInstanceName, ruleName, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup instance")
  cmd.Flags().String("rule-name", "", "Name of the backup rule (e.g., BackupWeekly)")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  cmd.MarkFlagRequired("rule-name")
  return cmd
}

func AdhocBackup(ctx context.Context, cmd *cobra.Command, resourceGroup, vaultName, backupInstanceName, ruleName string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  triggerOption := armdataprotection.TriggerBackupRequest{
    BackupRuleOptions: &armdataprotection.AdHocBackupRuleOptions{
      RuleName: &ruleName,
      TriggerOption: &armdataprotection.AdhocBasedTriggerContext{
        ObjectType: to.Ptr("AdhocBasedTriggerContext"),
      },
    },
  }

  poller, err := client.BeginAdhocBackup(ctx, resourceGroup, vaultName, backupInstanceName, triggerOption, nil)
  if err != nil {
    return fmt.Errorf("failed to trigger adhoc backup: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Backup operation started. Use 'az dataprotection job list' to monitor progress."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("adhoc backup operation failed: %w", err)
  }

  return output.PrintJSON(cmd, result)
}
```

- [ ] **Step 2: Register in commands.go**

Add `cmd.AddCommand(newAdhocBackupCommand())` in `NewBackupInstanceCommand()`.

- [ ] **Step 3: Build and verify**

```bash
make build && ./bin/az/az dataprotection backup-instance adhoc-backup --help
```

- [ ] **Step 4: Commit**

```bash
git add internal/dataprotection/backupinstance/adhocbackup.go internal/dataprotection/backupinstance/commands.go
git commit -m "feat: add dataprotection backup-instance adhoc-backup (TEC-2915)"
```

---

### Task 6: Backup Instance — CRUD (create, show, list, delete)

**Files:**
- Create: `internal/dataprotection/backupinstance/create.go`
- Create: `internal/dataprotection/backupinstance/show.go`
- Create: `internal/dataprotection/backupinstance/list.go`
- Create: `internal/dataprotection/backupinstance/delete.go`
- Modify: `internal/dataprotection/backupinstance/commands.go`

- [ ] **Step 1: Implement create**

Create `internal/dataprotection/backupinstance/create.go`:

```go
package backupinstance

import (
  "context"
  "encoding/json"
  "fmt"
  "os"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newCreateCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "create",
    Short: "Configure backup for a resource",
    Long:  "Creates a backup instance in a backup vault, enabling protection for the resource",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      backupInstanceFile, _ := cmd.Flags().GetString("backup-instance")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return CreateBackupInstance(context.Background(), resourceGroup, vaultName, backupInstanceFile, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("backup-instance", "", "Path to JSON file containing backup instance definition")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("backup-instance")
  return cmd
}

func CreateBackupInstance(ctx context.Context, resourceGroup, vaultName, backupInstanceFile string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  data, err := os.ReadFile(backupInstanceFile)
  if err != nil {
    return fmt.Errorf("failed to read backup instance file %s: %w", backupInstanceFile, err)
  }

  var instanceResource armdataprotection.BackupInstanceResource
  if err := json.Unmarshal(data, &instanceResource); err != nil {
    return fmt.Errorf("failed to parse backup instance JSON: %w", err)
  }

  instanceName := ""
  if instanceResource.Name != nil {
    instanceName = *instanceResource.Name
  } else if instanceResource.Properties != nil && instanceResource.Properties.FriendlyName != nil {
    instanceName = *instanceResource.Properties.FriendlyName
  } else {
    return fmt.Errorf("backup instance JSON must contain 'name' or 'properties.friendlyName'")
  }

  poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, vaultName, instanceName, instanceResource, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instance: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Backup instance creation started."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("backup instance creation failed: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 2: Implement show**

Create `internal/dataprotection/backupinstance/show.go`:

```go
package backupinstance

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newShowCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a backup instance",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      return ShowBackupInstance(context.Background(), resourceGroup, vaultName, name)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup instance")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  return cmd
}

func ShowBackupInstance(ctx context.Context, resourceGroup, vaultName, name string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  result, err := client.Get(ctx, resourceGroup, vaultName, name, nil)
  if err != nil {
    return fmt.Errorf("failed to get backup instance: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format backup instance: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 3: Implement list**

Create `internal/dataprotection/backupinstance/list.go`:

```go
package backupinstance

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List backup instances in a vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      return ListBackupInstances(context.Background(), resourceGroup, vaultName)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  return cmd
}

func ListBackupInstances(ctx context.Context, resourceGroup, vaultName string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  var instances []*armdataprotection.BackupInstanceResource
  pager := client.NewListPager(resourceGroup, vaultName, nil)
  for pager.More() {
    page, err := pager.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("failed to list backup instances: %w", err)
    }
    instances = append(instances, page.Value...)
  }

  output, err := json.MarshalIndent(instances, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format backup instances: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 4: Implement delete**

Create `internal/dataprotection/backupinstance/delete.go`:

```go
package backupinstance

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newDeleteCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a backup instance",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return DeleteBackupInstance(context.Background(), resourceGroup, vaultName, name, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup instance")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  return cmd
}

func DeleteBackupInstance(ctx context.Context, resourceGroup, vaultName, name string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  poller, err := client.BeginDelete(ctx, resourceGroup, vaultName, name, nil)
  if err != nil {
    return fmt.Errorf("failed to delete backup instance: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Delete operation started."}`)
    return nil
  }

  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("delete operation failed: %w", err)
  }

  fmt.Println(`{"status": "Backup instance deleted successfully."}`)
  return nil
}
```

- [ ] **Step 5: Register all CRUD commands in commands.go**

Update `NewBackupInstanceCommand()` to add: `newCreateCommand()`, `newShowCommand()`, `newListCommand()`, `newDeleteCommand()`.

- [ ] **Step 6: Build and verify**

```bash
make build && ./bin/az/az dataprotection backup-instance --help
```

Expected: shows create, show, list, delete, adhoc-backup, validate-for-restore, restore subcommands.

- [ ] **Step 7: Commit**

```bash
git add internal/dataprotection/backupinstance/
git commit -m "feat: add dataprotection backup-instance CRUD commands (TEC-2915)"
```

---

### Task 7: Backup Instance — Validate for Backup

**Files:**
- Create: `internal/dataprotection/backupinstance/validateforbackup.go`
- Modify: `internal/dataprotection/backupinstance/commands.go`

- [ ] **Step 1: Implement validate-for-backup**

Create `internal/dataprotection/backupinstance/validateforbackup.go`:

```go
package backupinstance

import (
  "context"
  "encoding/json"
  "fmt"
  "os"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newValidateForBackupCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "validate-for-backup",
    Short: "Validate a backup instance configuration",
    Long:  "Validates whether backup can be configured for a resource with the given backup instance definition",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      backupInstanceFile, _ := cmd.Flags().GetString("backup-instance")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return ValidateForBackup(context.Background(), resourceGroup, vaultName, backupInstanceFile, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("backup-instance", "", "Path to JSON file containing backup instance definition")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("backup-instance")
  return cmd
}

func ValidateForBackup(ctx context.Context, resourceGroup, vaultName, backupInstanceFile string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  data, err := os.ReadFile(backupInstanceFile)
  if err != nil {
    return fmt.Errorf("failed to read backup instance file %s: %w", backupInstanceFile, err)
  }

  var instanceResource armdataprotection.BackupInstanceResource
  if err := json.Unmarshal(data, &instanceResource); err != nil {
    return fmt.Errorf("failed to parse backup instance JSON: %w", err)
  }

  validateRequest := armdataprotection.ValidateForBackupRequest{
    BackupInstance: instanceResource.Properties,
  }

  poller, err := client.BeginValidateForBackup(ctx, resourceGroup, vaultName, validateRequest, nil)
  if err != nil {
    return fmt.Errorf("failed to validate for backup: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Validation started."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("backup validation failed: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format validation result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 2: Register in commands.go**

Add `cmd.AddCommand(newValidateForBackupCommand())`.

- [ ] **Step 3: Build and verify**

```bash
make build && ./bin/az/az dataprotection backup-instance validate-for-backup --help
```

- [ ] **Step 4: Commit**

```bash
git add internal/dataprotection/backupinstance/validateforbackup.go internal/dataprotection/backupinstance/commands.go
git commit -m "feat: add dataprotection backup-instance validate-for-backup (TEC-2915)"
```

---

### Task 8: Backup Instance — Lifecycle Commands (stop, suspend, resume)

**Files:**
- Create: `internal/dataprotection/backupinstance/stopprotection.go`
- Create: `internal/dataprotection/backupinstance/suspendbackup.go`
- Create: `internal/dataprotection/backupinstance/resumeprotection.go`
- Modify: `internal/dataprotection/backupinstance/commands.go`

- [ ] **Step 1: Implement stop-protection**

Create `internal/dataprotection/backupinstance/stopprotection.go`:

```go
package backupinstance

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newStopProtectionCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "stop-protection",
    Short: "Stop protection for a backup instance",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return StopProtection(context.Background(), resourceGroup, vaultName, name, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup instance")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  return cmd
}

func StopProtection(ctx context.Context, resourceGroup, vaultName, name string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  poller, err := client.BeginStopProtection(ctx, resourceGroup, vaultName, name, nil)
  if err != nil {
    return fmt.Errorf("failed to stop protection: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Stop protection operation started."}`)
    return nil
  }

  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("stop protection failed: %w", err)
  }

  fmt.Println(`{"status": "Protection stopped successfully."}`)
  return nil
}
```

- [ ] **Step 2: Implement suspend-backup**

Create `internal/dataprotection/backupinstance/suspendbackup.go`:

```go
package backupinstance

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newSuspendBackupCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "suspend-backup",
    Short: "Suspend backups for a backup instance",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return SuspendBackup(context.Background(), resourceGroup, vaultName, name, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup instance")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  return cmd
}

func SuspendBackup(ctx context.Context, resourceGroup, vaultName, name string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  poller, err := client.BeginSuspendBackups(ctx, resourceGroup, vaultName, name, nil)
  if err != nil {
    return fmt.Errorf("failed to suspend backups: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Suspend backup operation started."}`)
    return nil
  }

  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("suspend backup failed: %w", err)
  }

  fmt.Println(`{"status": "Backups suspended successfully."}`)
  return nil
}
```

- [ ] **Step 3: Implement resume-protection**

Create `internal/dataprotection/backupinstance/resumeprotection.go`:

```go
package backupinstance

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newResumeProtectionCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "resume-protection",
    Short: "Resume protection for a backup instance",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return ResumeProtection(context.Background(), resourceGroup, vaultName, name, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup instance")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  return cmd
}

func ResumeProtection(ctx context.Context, resourceGroup, vaultName, name string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupInstancesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup instances client: %w", err)
  }

  poller, err := client.BeginResumeProtection(ctx, resourceGroup, vaultName, name, nil)
  if err != nil {
    return fmt.Errorf("failed to resume protection: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Resume protection operation started."}`)
    return nil
  }

  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("resume protection failed: %w", err)
  }

  fmt.Println(`{"status": "Protection resumed successfully."}`)
  return nil
}
```

- [ ] **Step 4: Register all lifecycle commands**

Add `newStopProtectionCommand()`, `newSuspendBackupCommand()`, `newResumeProtectionCommand()` to `NewBackupInstanceCommand()`.

- [ ] **Step 5: Build and verify**

```bash
make build && ./bin/az/az dataprotection backup-instance --help
```

- [ ] **Step 6: Commit**

```bash
git add internal/dataprotection/backupinstance/
git commit -m "feat: add dataprotection backup-instance lifecycle commands (TEC-2915)

Adds stop-protection, suspend-backup, and resume-protection."
```

---

### Task 9: Backup Vault — CRUD

**Files:**
- Create: `internal/dataprotection/backupvault/commands.go`
- Create: `internal/dataprotection/backupvault/create.go`
- Create: `internal/dataprotection/backupvault/show.go`
- Create: `internal/dataprotection/backupvault/list.go`
- Create: `internal/dataprotection/backupvault/update.go`
- Create: `internal/dataprotection/backupvault/delete.go`
- Modify: `internal/dataprotection/commands.go`

- [ ] **Step 1: Create commands.go**

Create `internal/dataprotection/backupvault/commands.go`:

```go
package backupvault

import (
  "github.com/spf13/cobra"
)

func NewBackupVaultCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "backup-vault",
    Short: "Manage backup vaults",
    Long:  "Commands to manage Azure Data Protection backup vaults",
  }

  cmd.AddCommand(
    newCreateCommand(),
    newShowCommand(),
    newListCommand(),
    newUpdateCommand(),
    newDeleteCommand(),
  )
  return cmd
}
```

- [ ] **Step 2: Implement create**

Create `internal/dataprotection/backupvault/create.go`:

```go
package backupvault

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newCreateCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "create",
    Short: "Create a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      location, _ := cmd.Flags().GetString("location")
      datastoreType, _ := cmd.Flags().GetString("datastore-type")
      redundancy, _ := cmd.Flags().GetString("type")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return CreateVault(context.Background(), resourceGroup, vaultName, location, datastoreType, redundancy, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().StringP("vault-name", "v", "", "Name of the backup vault")
  cmd.Flags().StringP("location", "l", "", "Location (e.g., eastus)")
  cmd.Flags().String("datastore-type", "VaultStore", "Type of datastore (VaultStore, ArchiveStore)")
  cmd.Flags().String("type", "LocallyRedundant", "Storage redundancy type (LocallyRedundant, GeoRedundant, ZoneRedundant)")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("location")
  return cmd
}

func CreateVault(ctx context.Context, resourceGroup, vaultName, location, datastoreType, redundancy string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupVaultsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup vaults client: %w", err)
  }

  storageType := armdataprotection.StorageSettingStoreTypes(datastoreType)
  storageRedundancy := armdataprotection.StorageSettingTypes(redundancy)

  vault := armdataprotection.BackupVaultResource{
    Location: &location,
    Properties: &armdataprotection.BackupVault{
      StorageSettings: []*armdataprotection.StorageSetting{
        {
          DatastoreType: &storageType,
          Type:          &storageRedundancy,
        },
      },
    },
  }

  poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, vaultName, vault, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup vault: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Backup vault creation started."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("backup vault creation failed: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 3: Implement show**

Create `internal/dataprotection/backupvault/show.go`:

```go
package backupvault

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newShowCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      return ShowVault(context.Background(), resourceGroup, vaultName)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().StringP("vault-name", "v", "", "Name of the backup vault")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  return cmd
}

func ShowVault(ctx context.Context, resourceGroup, vaultName string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupVaultsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup vaults client: %w", err)
  }

  result, err := client.Get(ctx, resourceGroup, vaultName, nil)
  if err != nil {
    return fmt.Errorf("failed to get backup vault: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format backup vault: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 4: Implement list**

Create `internal/dataprotection/backupvault/list.go`:

```go
package backupvault

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List backup vaults",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      return ListVaults(context.Background(), resourceGroup)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all in subscription if not specified)")
  return cmd
}

func ListVaults(ctx context.Context, resourceGroup string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupVaultsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup vaults client: %w", err)
  }

  var vaults []*armdataprotection.BackupVaultResource

  if resourceGroup != "" {
    pager := client.NewGetInResourceGroupPager(resourceGroup, nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list backup vaults: %w", err)
      }
      vaults = append(vaults, page.Value...)
    }
  } else {
    pager := client.NewGetInSubscriptionPager(nil)
    for pager.More() {
      page, err := pager.NextPage(ctx)
      if err != nil {
        return fmt.Errorf("failed to list backup vaults: %w", err)
      }
      vaults = append(vaults, page.Value...)
    }
  }

  output, err := json.MarshalIndent(vaults, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format backup vaults: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 5: Implement update**

Create `internal/dataprotection/backupvault/update.go`:

```go
package backupvault

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newUpdateCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "update",
    Short: "Update a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      tags, _ := cmd.Flags().GetStringToString("tags")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return UpdateVault(context.Background(), resourceGroup, vaultName, tags, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().StringP("vault-name", "v", "", "Name of the backup vault")
  cmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  return cmd
}

func UpdateVault(ctx context.Context, resourceGroup, vaultName string, tags map[string]string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupVaultsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup vaults client: %w", err)
  }

  patchResource := armdataprotection.PatchResourceRequestInput{}
  if len(tags) > 0 {
    tagPtrs := make(map[string]*string)
    for k, v := range tags {
      val := v
      tagPtrs[k] = &val
    }
    patchResource.Tags = tagPtrs
  }

  poller, err := client.BeginUpdate(ctx, resourceGroup, vaultName, patchResource, nil)
  if err != nil {
    return fmt.Errorf("failed to update backup vault: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Update operation started."}`)
    return nil
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("backup vault update failed: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 6: Implement delete**

Create `internal/dataprotection/backupvault/delete.go`:

```go
package backupvault

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newDeleteCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a backup vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      noWait, _ := cmd.Flags().GetBool("no-wait")
      return DeleteVault(context.Background(), resourceGroup, vaultName, noWait)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().StringP("vault-name", "v", "", "Name of the backup vault")
  cmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  return cmd
}

func DeleteVault(ctx context.Context, resourceGroup, vaultName string, noWait bool) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupVaultsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup vaults client: %w", err)
  }

  poller, err := client.BeginDelete(ctx, resourceGroup, vaultName, nil)
  if err != nil {
    return fmt.Errorf("failed to delete backup vault: %w", err)
  }

  if noWait {
    fmt.Println(`{"status": "Delete operation started."}`)
    return nil
  }

  _, err = poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("delete operation failed: %w", err)
  }

  fmt.Println(`{"status": "Backup vault deleted successfully."}`)
  return nil
}
```

- [ ] **Step 7: Wire into root dataprotection command**

Update `internal/dataprotection/commands.go` to import `backupvault` and add `cmd.AddCommand(backupvault.NewBackupVaultCommand())`.

- [ ] **Step 8: Build and verify**

```bash
make build && ./bin/az/az dataprotection backup-vault --help
```

- [ ] **Step 9: Commit**

```bash
git add internal/dataprotection/backupvault/ internal/dataprotection/commands.go
git commit -m "feat: add dataprotection backup-vault CRUD commands (TEC-2915)"
```

---

### Task 10: Backup Policy — CRUD + Default Template

**Files:**
- Create: `internal/dataprotection/backuppolicy/commands.go`
- Create: `internal/dataprotection/backuppolicy/create.go`
- Create: `internal/dataprotection/backuppolicy/show.go`
- Create: `internal/dataprotection/backuppolicy/list.go`
- Create: `internal/dataprotection/backuppolicy/delete.go`
- Create: `internal/dataprotection/backuppolicy/defaultpolicy.go`
- Modify: `internal/dataprotection/commands.go`

- [ ] **Step 1: Create commands.go**

Create `internal/dataprotection/backuppolicy/commands.go`:

```go
package backuppolicy

import (
  "github.com/spf13/cobra"
)

func NewBackupPolicyCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "backup-policy",
    Short: "Manage backup policies",
    Long:  "Commands to manage backup policies within a backup vault",
  }

  cmd.AddCommand(
    newCreateCommand(),
    newShowCommand(),
    newListCommand(),
    newDeleteCommand(),
    newGetDefaultPolicyTemplateCommand(),
  )
  return cmd
}
```

- [ ] **Step 2: Implement create**

Create `internal/dataprotection/backuppolicy/create.go`:

```go
package backuppolicy

import (
  "context"
  "encoding/json"
  "fmt"
  "os"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newCreateCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "create",
    Short: "Create a backup policy",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      policyName, _ := cmd.Flags().GetString("name")
      policyFile, _ := cmd.Flags().GetString("policy")
      return CreatePolicy(context.Background(), resourceGroup, vaultName, policyName, policyFile)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup policy")
  cmd.Flags().String("policy", "", "Path to JSON file containing policy definition")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  cmd.MarkFlagRequired("policy")
  return cmd
}

func CreatePolicy(ctx context.Context, resourceGroup, vaultName, policyName, policyFile string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupPoliciesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup policies client: %w", err)
  }

  data, err := os.ReadFile(policyFile)
  if err != nil {
    return fmt.Errorf("failed to read policy file %s: %w", policyFile, err)
  }

  var policyResource armdataprotection.BaseBackupPolicyResource
  if err := json.Unmarshal(data, &policyResource); err != nil {
    return fmt.Errorf("failed to parse policy JSON: %w", err)
  }

  result, err := client.CreateOrUpdate(ctx, resourceGroup, vaultName, policyName, policyResource, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup policy: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format result: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 3: Implement show**

Create `internal/dataprotection/backuppolicy/show.go`:

```go
package backuppolicy

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newShowCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a backup policy",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      return ShowPolicy(context.Background(), resourceGroup, vaultName, name)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup policy")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  return cmd
}

func ShowPolicy(ctx context.Context, resourceGroup, vaultName, name string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupPoliciesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup policies client: %w", err)
  }

  result, err := client.Get(ctx, resourceGroup, vaultName, name, nil)
  if err != nil {
    return fmt.Errorf("failed to get backup policy: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format backup policy: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 4: Implement list**

Create `internal/dataprotection/backuppolicy/list.go`:

```go
package backuppolicy

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List backup policies in a vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      return ListPolicies(context.Background(), resourceGroup, vaultName)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  return cmd
}

func ListPolicies(ctx context.Context, resourceGroup, vaultName string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupPoliciesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup policies client: %w", err)
  }

  var policies []*armdataprotection.BaseBackupPolicyResource
  pager := client.NewListPager(resourceGroup, vaultName, nil)
  for pager.More() {
    page, err := pager.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("failed to list backup policies: %w", err)
    }
    policies = append(policies, page.Value...)
  }

  output, err := json.MarshalIndent(policies, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format backup policies: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 5: Implement delete**

Create `internal/dataprotection/backuppolicy/delete.go`:

```go
package backuppolicy

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newDeleteCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "delete",
    Short: "Delete a backup policy",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      name, _ := cmd.Flags().GetString("name")
      return DeletePolicy(context.Background(), resourceGroup, vaultName, name)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().StringP("name", "n", "", "Name of the backup policy")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("name")
  return cmd
}

func DeletePolicy(ctx context.Context, resourceGroup, vaultName, name string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewBackupPoliciesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create backup policies client: %w", err)
  }

  _, err = client.Delete(ctx, resourceGroup, vaultName, name, nil)
  if err != nil {
    return fmt.Errorf("failed to delete backup policy: %w", err)
  }

  fmt.Println(`{"status": "Backup policy deleted successfully."}`)
  return nil
}
```

- [ ] **Step 6: Implement get-default-policy-template**

Create `internal/dataprotection/backuppolicy/defaultpolicy.go`:

```go
package backuppolicy

import (
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/spf13/cobra"
)

func newGetDefaultPolicyTemplateCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "get-default-policy-template",
    Short: "Get the default policy template for a datasource type",
    Long:  "Returns a default backup policy JSON template for the specified datasource type",
    RunE: func(cmd *cobra.Command, args []string) error {
      datasourceType, _ := cmd.Flags().GetString("datasource-type")
      return GetDefaultPolicyTemplate(datasourceType)
    },
  }
  cmd.Flags().String("datasource-type", "", "Datasource type (e.g., AzureDatabaseForPostgreSQLFlexibleServer, AzureBlob, AzureDisk, AzureKubernetesService)")
  cmd.MarkFlagRequired("datasource-type")
  return cmd
}

func GetDefaultPolicyTemplate(datasourceType string) error {
  template, err := buildDefaultTemplate(datasourceType)
  if err != nil {
    return err
  }

  output, err := json.MarshalIndent(template, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format policy template: %w", err)
  }

  fmt.Println(string(output))
  return nil
}

func buildDefaultTemplate(datasourceType string) (*armdataprotection.BaseBackupPolicyResource, error) {
  switch datasourceType {
  case "AzureDatabaseForPostgreSQLFlexibleServer":
    return buildPGFlexDefaultPolicy(), nil
  default:
    return nil, fmt.Errorf("unsupported datasource type: %s. Supported types: AzureDatabaseForPostgreSQLFlexibleServer", datasourceType)
  }
}

func buildPGFlexDefaultPolicy() *armdataprotection.BaseBackupPolicyResource {
  backupSchedule := "R/2024-01-01T00:00:00+00:00/P1W" // Weekly
  vaultStore := armdataprotection.DataStoreTypesVaultStore

  return &armdataprotection.BaseBackupPolicyResource{
    Properties: &armdataprotection.BackupPolicy{
      ObjectType:     to.Ptr("BackupPolicy"),
      DatasourceTypes: []*string{to.Ptr("Microsoft.DBforPostgreSQL/flexibleServers")},
      PolicyRules: []armdataprotection.BasePolicyRuleClassification{
        &armdataprotection.AzureBackupRule{
          Name:       to.Ptr("BackupWeekly"),
          ObjectType: to.Ptr("AzureBackupRule"),
          DataStore: &armdataprotection.DataStoreInfoBase{
            DataStoreType: &vaultStore,
            ObjectType:    to.Ptr("DataStoreInfoBase"),
          },
          BackupParameters: &armdataprotection.AzureBackupParams{
            ObjectType:    to.Ptr("AzureBackupParams"),
            BackupType:    to.Ptr("Full"),
          },
          Trigger: &armdataprotection.ScheduleBasedTriggerContext{
            ObjectType: to.Ptr("ScheduleBasedTriggerContext"),
            Schedule: &armdataprotection.BackupSchedule{
              RepeatingTimeIntervals: []*string{&backupSchedule},
            },
            TaggingCriteria: []*armdataprotection.TaggingCriteria{
              {
                IsDefault:       to.Ptr(true),
                TaggingPriority: to.Ptr[int64](99),
                TagInfo: &armdataprotection.RetentionTag{
                  TagName: to.Ptr("Default"),
                },
              },
            },
          },
        },
        &armdataprotection.AzureRetentionRule{
          Name:       to.Ptr("Default"),
          ObjectType: to.Ptr("AzureRetentionRule"),
          IsDefault:  to.Ptr(true),
          Lifecycles: []*armdataprotection.SourceLifeCycle{
            {
              DeleteAfter: &armdataprotection.AbsoluteDeleteOption{
                ObjectType: to.Ptr("AbsoluteDeleteOption"),
                Duration:   to.Ptr("P3M"),
              },
              SourceDataStore: &armdataprotection.DataStoreInfoBase{
                DataStoreType: &vaultStore,
                ObjectType:    to.Ptr("DataStoreInfoBase"),
              },
            },
          },
        },
      },
    },
  }
}
```

- [ ] **Step 7: Wire into root dataprotection command**

Update `internal/dataprotection/commands.go` to import `backuppolicy` and add `cmd.AddCommand(backuppolicy.NewBackupPolicyCommand())`.

- [ ] **Step 8: Build and verify**

```bash
make build && ./bin/az/az dataprotection backup-policy --help && ./bin/az/az dataprotection backup-policy get-default-policy-template --datasource-type AzureDatabaseForPostgreSQLFlexibleServer
```

Expected: policy template JSON output for PG Flex.

- [ ] **Step 9: Commit**

```bash
git add internal/dataprotection/backuppolicy/ internal/dataprotection/commands.go
git commit -m "feat: add dataprotection backup-policy commands with PG Flex default template (TEC-2915)"
```

---

### Task 11: Recovery Point — List and Show

**Files:**
- Create: `internal/dataprotection/recoverypoint/commands.go`
- Create: `internal/dataprotection/recoverypoint/list.go`
- Create: `internal/dataprotection/recoverypoint/show.go`
- Modify: `internal/dataprotection/commands.go`

- [ ] **Step 1: Create commands.go**

Create `internal/dataprotection/recoverypoint/commands.go`:

```go
package recoverypoint

import (
  "github.com/spf13/cobra"
)

func NewRecoveryPointCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "recovery-point",
    Short: "Manage recovery points",
    Long:  "Commands to manage recovery points for backup instances",
  }

  cmd.AddCommand(
    newListCommand(),
    newShowCommand(),
  )
  return cmd
}
```

- [ ] **Step 2: Implement list**

Create `internal/dataprotection/recoverypoint/list.go`:

```go
package recoverypoint

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List recovery points for a backup instance",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      backupInstanceName, _ := cmd.Flags().GetString("backup-instance-name")
      return ListRecoveryPoints(context.Background(), resourceGroup, vaultName, backupInstanceName)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("backup-instance-name", "", "Name of the backup instance")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("backup-instance-name")
  return cmd
}

func ListRecoveryPoints(ctx context.Context, resourceGroup, vaultName, backupInstanceName string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewRecoveryPointsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create recovery points client: %w", err)
  }

  var points []*armdataprotection.AzureBackupRecoveryPointResource
  pager := client.NewListPager(resourceGroup, vaultName, backupInstanceName, nil)
  for pager.More() {
    page, err := pager.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("failed to list recovery points: %w", err)
    }
    points = append(points, page.Value...)
  }

  output, err := json.MarshalIndent(points, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format recovery points: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 3: Implement show**

Create `internal/dataprotection/recoverypoint/show.go`:

```go
package recoverypoint

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newShowCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a recovery point",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      backupInstanceName, _ := cmd.Flags().GetString("backup-instance-name")
      recoveryPointID, _ := cmd.Flags().GetString("recovery-point-id")
      return ShowRecoveryPoint(context.Background(), resourceGroup, vaultName, backupInstanceName, recoveryPointID)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("backup-instance-name", "", "Name of the backup instance")
  cmd.Flags().String("recovery-point-id", "", "ID of the recovery point")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("backup-instance-name")
  cmd.MarkFlagRequired("recovery-point-id")
  return cmd
}

func ShowRecoveryPoint(ctx context.Context, resourceGroup, vaultName, backupInstanceName, recoveryPointID string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewRecoveryPointsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create recovery points client: %w", err)
  }

  result, err := client.Get(ctx, resourceGroup, vaultName, backupInstanceName, recoveryPointID, nil)
  if err != nil {
    return fmt.Errorf("failed to get recovery point: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format recovery point: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 4: Wire into root dataprotection command**

Update `internal/dataprotection/commands.go` to import `recoverypoint` and add `cmd.AddCommand(recoverypoint.NewRecoveryPointCommand())`.

- [ ] **Step 5: Build and verify**

```bash
make build && ./bin/az/az dataprotection recovery-point --help
```

- [ ] **Step 6: Commit**

```bash
git add internal/dataprotection/recoverypoint/ internal/dataprotection/commands.go
git commit -m "feat: add dataprotection recovery-point list and show commands (TEC-2915)"
```

---

### Task 12: Job — List and Show

**Files:**
- Create: `internal/dataprotection/job/commands.go`
- Create: `internal/dataprotection/job/list.go`
- Create: `internal/dataprotection/job/show.go`
- Modify: `internal/dataprotection/commands.go`

- [ ] **Step 1: Create commands.go**

Create `internal/dataprotection/job/commands.go`:

```go
package job

import (
  "github.com/spf13/cobra"
)

func NewJobCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "job",
    Short: "Manage backup and restore jobs",
    Long:  "Commands to monitor backup and restore job status",
  }

  cmd.AddCommand(
    newListCommand(),
    newShowCommand(),
  )
  return cmd
}
```

- [ ] **Step 2: Implement list**

Create `internal/dataprotection/job/list.go`:

```go
package job

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "list",
    Short: "List backup and restore jobs in a vault",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      return ListJobs(context.Background(), resourceGroup, vaultName)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  return cmd
}

func ListJobs(ctx context.Context, resourceGroup, vaultName string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewJobsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create jobs client: %w", err)
  }

  var jobs []*armdataprotection.AzureBackupJobResource
  pager := client.NewListPager(resourceGroup, vaultName, nil)
  for pager.More() {
    page, err := pager.NextPage(ctx)
    if err != nil {
      return fmt.Errorf("failed to list jobs: %w", err)
    }
    jobs = append(jobs, page.Value...)
  }

  output, err := json.MarshalIndent(jobs, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format jobs: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 3: Implement show**

Create `internal/dataprotection/job/show.go`:

```go
package job

import (
  "context"
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/spf13/cobra"
)

func newShowCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "show",
    Short: "Show details of a backup or restore job",
    RunE: func(cmd *cobra.Command, args []string) error {
      resourceGroup, _ := cmd.Flags().GetString("resource-group")
      vaultName, _ := cmd.Flags().GetString("vault-name")
      jobID, _ := cmd.Flags().GetString("job-id")
      return ShowJob(context.Background(), resourceGroup, vaultName, jobID)
    },
  }
  cmd.Flags().StringP("resource-group", "g", "", "Name of resource group")
  cmd.Flags().String("vault-name", "", "Name of the backup vault")
  cmd.Flags().String("job-id", "", "ID of the job")
  cmd.MarkFlagRequired("resource-group")
  cmd.MarkFlagRequired("vault-name")
  cmd.MarkFlagRequired("job-id")
  return cmd
}

func ShowJob(ctx context.Context, resourceGroup, vaultName, jobID string) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return err
  }

  client, err := armdataprotection.NewJobsClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create jobs client: %w", err)
  }

  result, err := client.Get(ctx, resourceGroup, vaultName, jobID, nil)
  if err != nil {
    return fmt.Errorf("failed to get job: %w", err)
  }

  output, err := json.MarshalIndent(result, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format job: %w", err)
  }

  fmt.Println(string(output))
  return nil
}
```

- [ ] **Step 4: Wire into root dataprotection command**

Update `internal/dataprotection/commands.go` to import `job` and add `cmd.AddCommand(job.NewJobCommand())`.

- [ ] **Step 5: Build and verify**

```bash
make build && ./bin/az/az dataprotection job --help
```

- [ ] **Step 6: Commit**

```bash
git add internal/dataprotection/job/ internal/dataprotection/commands.go
git commit -m "feat: add dataprotection job list and show commands (TEC-2915)"
```

---

### Task 13: Final Build + Full Command Tree Verification

**Files:** None (verification only)

- [ ] **Step 1: Run go mod tidy**

```bash
cd /Users/christopherdobbyn/work/dobbo-ca/azure-go-cli && go mod tidy
```

- [ ] **Step 2: Run tests**

```bash
make test
```

- [ ] **Step 3: Build**

```bash
make build
```

- [ ] **Step 4: Verify full command tree**

```bash
./bin/az/az dataprotection --help
./bin/az/az dataprotection backup-vault --help
./bin/az/az dataprotection backup-policy --help
./bin/az/az dataprotection backup-instance --help
./bin/az/az dataprotection backup-instance restore --help
./bin/az/az dataprotection recovery-point --help
./bin/az/az dataprotection job --help
```

- [ ] **Step 5: Commit any tidy changes**

```bash
git add go.mod go.sum
git commit -m "chore: go mod tidy after dataprotection implementation (TEC-2915)"
```
