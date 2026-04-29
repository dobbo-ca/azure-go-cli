# vnet subnet path + subnet update + route-table commands — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `az network vnet subnet ...` work (matching official az CLI), keep `az network subnet ...` as an alias path, add `subnet update` for attach/detach of NSG/route-table/NAT-gateway/service-endpoints/delegations, and add full `route-table` and `route-table route` command groups — all using the existing `pkg/output.PrintJSON` so `--query` works.

**Architecture:** Single factory pattern. `subnet.NewSubnetCommand()` returns a fresh cobra tree on each call and is mounted under both `network` and `network vnet`. New `routetable` package mirrors existing peering/lb/nic patterns. Nested `route-table route` lives in `internal/network/routetable/route/`. All read commands route output through `pkg/output.PrintJSON(cmd, data)`.

**Tech Stack:** Go, cobra, `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6`.

**Note on tests:** Existing `internal/network/*` packages have no test files. Per the spec, this work follows the existing pattern — verification is manual via `make build` + smoke commands. Each task ends with a `make build` step to catch compile errors.

---

## File map

**Modify:**
- `internal/network/commands.go` — add `routetable.NewRouteTableCommand()`
- `internal/network/subnet/commands.go` — add `update` subcommand, change `List`/`Show` signatures
- `internal/network/subnet/list.go` — use `pkg/output.PrintJSON`; add `addressPrefixes`
- `internal/network/subnet/show.go` — use `pkg/output.PrintJSON`
- `internal/network/vnet/commands.go` — mount `subnet.NewSubnetCommand()` as a subcommand

**Create:**
- `internal/network/subnet/update.go`
- `internal/network/routetable/commands.go`
- `internal/network/routetable/list.go`
- `internal/network/routetable/show.go`
- `internal/network/routetable/create.go`
- `internal/network/routetable/delete.go`
- `internal/network/routetable/route/commands.go`
- `internal/network/routetable/route/list.go`
- `internal/network/routetable/route/show.go`
- `internal/network/routetable/route/create.go`
- `internal/network/routetable/route/delete.go`

---

### Task 1: Refactor `subnet list` to use `pkg/output.PrintJSON` and emit `addressPrefixes`

**Files:**
- Modify: `internal/network/subnet/list.go`
- Modify: `internal/network/subnet/commands.go`

- [ ] **Step 1: Update `List` signature and body**

Replace the entire contents of `internal/network/subnet/list.go` with:

```go
package subnet

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, vnetName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armnetwork.NewSubnetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create subnets client: %w", err)
	}

	pager := client.NewListPager(resourceGroup, vnetName, nil)
	var subnets []map[string]interface{}

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list subnets: %w", err)
		}

		for _, subnet := range page.Value {
			subnets = append(subnets, formatSubnet(subnet))
		}
	}

	return output.PrintJSON(cmd, subnets)
}

func formatSubnet(subnet *armnetwork.Subnet) map[string]interface{} {
	result := map[string]interface{}{
		"name": azure.GetStringValue(subnet.Name),
	}

	if subnet.ID != nil {
		result["id"] = *subnet.ID
	}

	if subnet.Properties != nil {
		if subnet.Properties.AddressPrefix != nil {
			result["addressPrefix"] = *subnet.Properties.AddressPrefix
		}
		if len(subnet.Properties.AddressPrefixes) > 0 {
			prefixes := make([]string, 0, len(subnet.Properties.AddressPrefixes))
			for _, p := range subnet.Properties.AddressPrefixes {
				if p != nil {
					prefixes = append(prefixes, *p)
				}
			}
			result["addressPrefixes"] = prefixes
		}
		if subnet.Properties.ProvisioningState != nil {
			result["provisioningState"] = string(*subnet.Properties.ProvisioningState)
		}
		if subnet.Properties.NetworkSecurityGroup != nil {
			nsg := map[string]interface{}{}
			if subnet.Properties.NetworkSecurityGroup.ID != nil {
				nsg["id"] = *subnet.Properties.NetworkSecurityGroup.ID
			}
			result["networkSecurityGroup"] = nsg
		}
		if subnet.Properties.RouteTable != nil {
			rt := map[string]interface{}{}
			if subnet.Properties.RouteTable.ID != nil {
				rt["id"] = *subnet.Properties.RouteTable.ID
			}
			result["routeTable"] = rt
		}
		if subnet.Properties.NatGateway != nil {
			ng := map[string]interface{}{}
			if subnet.Properties.NatGateway.ID != nil {
				ng["id"] = *subnet.Properties.NatGateway.ID
			}
			result["natGateway"] = ng
		}
	}

	return result
}
```

> Note: `networkSecurityGroup`, `routeTable`, and `natGateway` are now nested objects with an `id` field, matching real az CLI output (so the user's `--query "...nsg:networkSecurityGroup.id"` syntax works).

- [ ] **Step 2: Update call site in `commands.go`**

In `internal/network/subnet/commands.go`, change the `listCmd.RunE` body from:

```go
return List(context.Background(), vnetName, resourceGroup)
```

to:

```go
return List(context.Background(), cmd, vnetName, resourceGroup)
```

- [ ] **Step 3: Build to confirm compile**

Run: `make build`
Expected: `Binary created: bin/az/az`

- [ ] **Step 4: Commit**

```bash
git add internal/network/subnet/list.go internal/network/subnet/commands.go
git commit -m "refactor(subnet): use pkg/output for list and emit nested NSG/route-table/NAT IDs"
```

---

### Task 2: Refactor `subnet show` to use `pkg/output.PrintJSON`

**Files:**
- Modify: `internal/network/subnet/show.go`
- Modify: `internal/network/subnet/commands.go`

- [ ] **Step 1: Update `Show` signature and body**

Replace the entire contents of `internal/network/subnet/show.go` with:

```go
package subnet

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, vnetName, subnetName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armnetwork.NewSubnetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create subnets client: %w", err)
	}

	resp, err := client.Get(ctx, resourceGroup, vnetName, subnetName, nil)
	if err != nil {
		return fmt.Errorf("failed to get subnet: %w", err)
	}

	return output.PrintJSON(cmd, resp.Subnet)
}
```

- [ ] **Step 2: Update call site in `commands.go`**

In `internal/network/subnet/commands.go`, change `showCmd.RunE` from:

```go
return Show(context.Background(), vnetName, subnetName, resourceGroup)
```

to:

```go
return Show(context.Background(), cmd, vnetName, subnetName, resourceGroup)
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/network/subnet/show.go internal/network/subnet/commands.go
git commit -m "refactor(subnet): use pkg/output for show"
```

---

### Task 3: Mount `subnet` as a subcommand of `vnet` (dual-mount)

**Files:**
- Modify: `internal/network/vnet/commands.go`

- [ ] **Step 1: Add import**

In `internal/network/vnet/commands.go`, add to the import block:

```go
"github.com/cdobbyn/azure-go-cli/internal/network/subnet"
```

- [ ] **Step 2: Mount subnet command**

Change the final `cmd.AddCommand` line from:

```go
cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
```

to:

```go
cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, subnet.NewSubnetCommand())
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Smoke check both paths exist**

Run: `./bin/az/az network vnet subnet --help`
Expected: shows `list`, `show`, `create`, `delete` subcommands.

Run: `./bin/az/az network subnet --help`
Expected: same subcommands (old path still works).

- [ ] **Step 5: Commit**

```bash
git add internal/network/vnet/commands.go
git commit -m "feat(network): mount subnet under vnet to match official az CLI path"
```

---

### Task 4: Add `subnet update` command

**Files:**
- Create: `internal/network/subnet/update.go`
- Modify: `internal/network/subnet/commands.go`

- [ ] **Step 1: Create `update.go`**

Create `internal/network/subnet/update.go` with this exact content:

```go
package subnet

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Update(ctx context.Context, cmd *cobra.Command, name, vnetName, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewSubnetsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create subnets client: %w", err)
	}

	current, err := client.Get(ctx, resourceGroup, vnetName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get subnet: %w", err)
	}

	if current.Properties == nil {
		current.Properties = &armnetwork.SubnetPropertiesFormat{}
	}
	props := current.Properties

	flags := cmd.Flags()

	if flags.Changed("network-security-group") {
		v, _ := flags.GetString("network-security-group")
		if v == "" {
			props.NetworkSecurityGroup = nil
		} else {
			props.NetworkSecurityGroup = &armnetwork.SecurityGroup{
				ID: to.Ptr(resolveNetworkResourceID(v, subscriptionID, resourceGroup, "networkSecurityGroups")),
			}
		}
	}

	if flags.Changed("route-table") {
		v, _ := flags.GetString("route-table")
		if v == "" {
			props.RouteTable = nil
		} else {
			props.RouteTable = &armnetwork.RouteTable{
				ID: to.Ptr(resolveNetworkResourceID(v, subscriptionID, resourceGroup, "routeTables")),
			}
		}
	}

	if flags.Changed("nat-gateway") {
		v, _ := flags.GetString("nat-gateway")
		if v == "" {
			props.NatGateway = nil
		} else {
			props.NatGateway = &armnetwork.SubResource{
				ID: to.Ptr(resolveNetworkResourceID(v, subscriptionID, resourceGroup, "natGateways")),
			}
		}
	}

	if flags.Changed("service-endpoints") {
		v, _ := flags.GetString("service-endpoints")
		if v == "" {
			props.ServiceEndpoints = nil
		} else {
			services := splitCSV(v)
			endpoints := make([]*armnetwork.ServiceEndpointPropertiesFormat, 0, len(services))
			for _, svc := range services {
				endpoints = append(endpoints, &armnetwork.ServiceEndpointPropertiesFormat{
					Service: to.Ptr(svc),
				})
			}
			props.ServiceEndpoints = endpoints
		}
	}

	if flags.Changed("delegations") {
		v, _ := flags.GetString("delegations")
		if v == "" {
			props.Delegations = nil
		} else {
			services := splitCSV(v)
			delegations := make([]*armnetwork.Delegation, 0, len(services))
			for _, svc := range services {
				// Use the service name as the delegation name (matches az CLI behavior)
				delegations = append(delegations, &armnetwork.Delegation{
					Name: to.Ptr(svc),
					Properties: &armnetwork.ServiceDelegationPropertiesFormat{
						ServiceName: to.Ptr(svc),
					},
				})
			}
			props.Delegations = delegations
		}
	}

	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, vnetName, name, current.Subnet, nil)
	if err != nil {
		return fmt.Errorf("failed to begin update subnet: %w", err)
	}

	if noWait {
		fmt.Printf("Started update of subnet '%s' in VNet '%s'\n", name, vnetName)
		return nil
	}

	fmt.Printf("Updating subnet '%s' in VNet '%s'...\n", name, vnetName)
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update subnet: %w", err)
	}

	return output.PrintJSON(cmd, result.Subnet)
}

// resolveNetworkResourceID returns v unchanged if it's already a full resource ID;
// otherwise constructs a Microsoft.Network resource ID using the given resource type
// (e.g., "networkSecurityGroups", "routeTables", "natGateways") in the same subscription
// and resource group as the subnet.
func resolveNetworkResourceID(v, subscriptionID, resourceGroup, resourceType string) string {
	if strings.HasPrefix(v, "/subscriptions/") {
		return v
	}
	return fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/%s/%s",
		subscriptionID, resourceGroup, resourceType, v,
	)
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
```

- [ ] **Step 2: Add `update` subcommand to `commands.go`**

In `internal/network/subnet/commands.go`, after the `deleteCmd` block and before `cmd.AddCommand(...)`, insert:

```go
	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a subnet (attach/detach NSG, route table, NAT gateway, service endpoints, delegations)",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			vnetName, _ := cmd.Flags().GetString("vnet-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Update(context.Background(), cmd, name, vnetName, resourceGroup, noWait)
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "Subnet name")
	updateCmd.Flags().String("vnet-name", "", "Virtual network name")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().String("network-security-group", "", "NSG resource ID or name (empty string clears)")
	updateCmd.Flags().String("route-table", "", "Route table resource ID or name (empty string clears)")
	updateCmd.Flags().String("nat-gateway", "", "NAT gateway resource ID or name (empty string clears)")
	updateCmd.Flags().String("service-endpoints", "", "Comma-separated service endpoints (e.g., Microsoft.Storage,Microsoft.KeyVault); empty string clears")
	updateCmd.Flags().String("delegations", "", "Comma-separated service delegations (e.g., Microsoft.ContainerInstance/containerGroups); empty string clears")
	updateCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	updateCmd.MarkFlagRequired("name")
	updateCmd.MarkFlagRequired("vnet-name")
	updateCmd.MarkFlagRequired("resource-group")
```

Then change the final line from:

```go
cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
```

to:

```go
cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, updateCmd)
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: `Binary created: bin/az/az`

- [ ] **Step 4: Smoke**

Run: `./bin/az/az network vnet subnet update --help`
Expected: shows all flags including `--network-security-group`, `--route-table`, `--nat-gateway`, `--service-endpoints`, `--delegations`, `--no-wait`.

- [ ] **Step 5: Commit**

```bash
git add internal/network/subnet/update.go internal/network/subnet/commands.go
git commit -m "feat(subnet): add update command for NSG, route-table, NAT, service-endpoints, delegations"
```

---

### Task 5: Create `routetable` package skeleton + `list`

**Files:**
- Create: `internal/network/routetable/commands.go`
- Create: `internal/network/routetable/list.go`

- [ ] **Step 1: Create `commands.go`**

Create `internal/network/routetable/commands.go` with:

```go
package routetable

import (
	"context"

	"github.com/spf13/cobra"
)

func NewRouteTableCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route-table",
		Short: "Manage route tables",
		Long:  "Commands to manage Azure route tables and their routes",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List route tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	cmd.AddCommand(listCmd)
	return cmd
}
```

- [ ] **Step 2: Create `list.go`**

Create `internal/network/routetable/list.go` with:

```go
package routetable

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, resourceGroup string) error {
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

	var tables []*armnetwork.RouteTable

	if resourceGroup != "" {
		pager := client.NewListPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list route tables: %w", err)
			}
			tables = append(tables, page.Value...)
		}
	} else {
		pager := client.NewListAllPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list route tables: %w", err)
			}
			tables = append(tables, page.Value...)
		}
	}

	return output.PrintJSON(cmd, tables)
}
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: success (the package compiles even though it's not yet wired into the network command).

- [ ] **Step 4: Commit**

```bash
git add internal/network/routetable/
git commit -m "feat(route-table): add list command"
```

---

### Task 6: Add `route-table show`

**Files:**
- Create: `internal/network/routetable/show.go`
- Modify: `internal/network/routetable/commands.go`

- [ ] **Step 1: Create `show.go`**

Create `internal/network/routetable/show.go` with:

```go
package routetable

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, name, resourceGroup string) error {
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

	resp, err := client.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get route table: %w", err)
	}

	return output.PrintJSON(cmd, resp.RouteTable)
}
```

- [ ] **Step 2: Add `show` subcommand**

In `internal/network/routetable/commands.go`, before the final `cmd.AddCommand(...)`, add:

```go
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a route table",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, name, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Route table name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")
```

Update the AddCommand line to:

```go
cmd.AddCommand(listCmd, showCmd)
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/network/routetable/show.go internal/network/routetable/commands.go
git commit -m "feat(route-table): add show command"
```

---

### Task 7: Add `route-table create`

**Files:**
- Create: `internal/network/routetable/create.go`
- Modify: `internal/network/routetable/commands.go`

- [ ] **Step 1: Create `create.go`**

Create `internal/network/routetable/create.go` with:

```go
package routetable

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, name, resourceGroup, location string, disableBGPRoutePropagation bool, tags map[string]string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewRouteTablesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create route tables client: %w", err)
	}

	tagPtrs := make(map[string]*string, len(tags))
	for k, v := range tags {
		v := v
		tagPtrs[k] = &v
	}

	parameters := armnetwork.RouteTable{
		Location: to.Ptr(location),
		Tags:     tagPtrs,
		Properties: &armnetwork.RouteTablePropertiesFormat{
			DisableBgpRoutePropagation: to.Ptr(disableBGPRoutePropagation),
		},
	}

	fmt.Printf("Creating route table '%s'...\n", name)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create route table: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create route table: %w", err)
	}

	return output.PrintJSON(cmd, result.RouteTable)
}
```

- [ ] **Step 2: Add `create` subcommand**

In `internal/network/routetable/commands.go`, before the final `cmd.AddCommand(...)`, add:

```go
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a route table",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			disableBGP, _ := cmd.Flags().GetBool("disable-bgp-route-propagation")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, resourceGroup, location, disableBGP, tags)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Route table name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().Bool("disable-bgp-route-propagation", false, "Disable BGP route propagation from the virtual network gateway")
	createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("location")
```

Update the AddCommand line to:

```go
cmd.AddCommand(listCmd, showCmd, createCmd)
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/network/routetable/create.go internal/network/routetable/commands.go
git commit -m "feat(route-table): add create command"
```

---

### Task 8: Add `route-table delete`

**Files:**
- Create: `internal/network/routetable/delete.go`
- Modify: `internal/network/routetable/commands.go`

- [ ] **Step 1: Create `delete.go`**

Create `internal/network/routetable/delete.go` with:

```go
package routetable

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, name, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewRouteTablesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create route tables client: %w", err)
	}

	poller, err := client.BeginDelete(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete route table: %w", err)
	}

	if noWait {
		fmt.Printf("Started deletion of route table '%s'\n", name)
		return nil
	}

	fmt.Printf("Deleting route table '%s'...\n", name)
	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to delete route table: %w", err)
	}

	fmt.Printf("Deleted route table '%s'\n", name)
	return nil
}
```

- [ ] **Step 2: Add `delete` subcommand**

In `internal/network/routetable/commands.go`, before the final `cmd.AddCommand(...)`, add:

```go
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a route table",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), name, resourceGroup, noWait)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Route table name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("resource-group")
```

Update the AddCommand line to:

```go
cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/network/routetable/delete.go internal/network/routetable/commands.go
git commit -m "feat(route-table): add delete command"
```

---

### Task 9: Wire `route-table` into the `network` command

**Files:**
- Modify: `internal/network/commands.go`

- [ ] **Step 1: Add import**

In `internal/network/commands.go`, add to the import block (preserving alphabetical order):

```go
"github.com/cdobbyn/azure-go-cli/internal/network/routetable"
```

- [ ] **Step 2: Mount it**

Add `routetable.NewRouteTableCommand(),` to the `cmd.AddCommand(...)` block. Place it next to `peering` for grouping. Final block becomes:

```go
cmd.AddCommand(
    bastion.NewBastionCommand(),
    vnet.NewVNetCommand(),
    subnet.NewSubnetCommand(),
    peering.NewPeeringCommand(),
    routetable.NewRouteTableCommand(),
    natgateway.NewNatGatewayCommand(),
    vpngateway.NewVpnGatewayCommand(),
    lb.NewLoadBalancerCommand(),
    privateendpoint.NewPrivateEndpointCommand(),
    nsg.NewNsgCommand(),
    publicip.NewPublicIPCommand(),
    nic.NewNicCommand(),
)
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Smoke**

Run: `./bin/az/az network route-table --help`
Expected: shows `list`, `show`, `create`, `delete`.

- [ ] **Step 5: Commit**

```bash
git add internal/network/commands.go
git commit -m "feat(network): mount route-table command group"
```

---

### Task 10: Create `route-table route` package skeleton + `list`

**Files:**
- Create: `internal/network/routetable/route/commands.go`
- Create: `internal/network/routetable/route/list.go`

- [ ] **Step 1: Create `commands.go`**

Create `internal/network/routetable/route/commands.go`:

```go
package route

import (
	"context"

	"github.com/spf13/cobra"
)

func NewRouteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Manage routes within a route table",
		Long:  "Commands to manage individual routes within an Azure route table",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List routes in a route table",
		RunE: func(cmd *cobra.Command, args []string) error {
			routeTableName, _ := cmd.Flags().GetString("route-table-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, routeTableName, resourceGroup)
		},
	}
	listCmd.Flags().String("route-table-name", "", "Route table name")
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	listCmd.MarkFlagRequired("route-table-name")
	listCmd.MarkFlagRequired("resource-group")

	cmd.AddCommand(listCmd)
	return cmd
}
```

- [ ] **Step 2: Create `list.go`**

Create `internal/network/routetable/route/list.go`:

```go
package route

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func List(ctx context.Context, cmd *cobra.Command, routeTableName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armnetwork.NewRoutesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create routes client: %w", err)
	}

	pager := client.NewListPager(resourceGroup, routeTableName, nil)
	var routes []*armnetwork.Route

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list routes: %w", err)
		}
		routes = append(routes, page.Value...)
	}

	return output.PrintJSON(cmd, routes)
}
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: success (the package compiles standalone).

- [ ] **Step 4: Commit**

```bash
git add internal/network/routetable/route/
git commit -m "feat(route-table): add nested route list command"
```

---

### Task 11: Add `route show`

**Files:**
- Create: `internal/network/routetable/route/show.go`
- Modify: `internal/network/routetable/route/commands.go`

- [ ] **Step 1: Create `show.go`**

Create `internal/network/routetable/route/show.go`:

```go
package route

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Show(ctx context.Context, cmd *cobra.Command, name, routeTableName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armnetwork.NewRoutesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create routes client: %w", err)
	}

	resp, err := client.Get(ctx, resourceGroup, routeTableName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get route: %w", err)
	}

	return output.PrintJSON(cmd, resp.Route)
}
```

- [ ] **Step 2: Add `show` subcommand**

In `internal/network/routetable/route/commands.go`, before the final `cmd.AddCommand(...)`, add:

```go
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a route",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			routeTableName, _ := cmd.Flags().GetString("route-table-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, name, routeTableName, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Route name")
	showCmd.Flags().String("route-table-name", "", "Route table name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("route-table-name")
	showCmd.MarkFlagRequired("resource-group")
```

Update AddCommand to: `cmd.AddCommand(listCmd, showCmd)`.

- [ ] **Step 3: Build + commit**

```bash
make build
git add internal/network/routetable/route/show.go internal/network/routetable/route/commands.go
git commit -m "feat(route-table): add nested route show command"
```

---

### Task 12: Add `route create`

**Files:**
- Create: `internal/network/routetable/route/create.go`
- Modify: `internal/network/routetable/route/commands.go`

- [ ] **Step 1: Create `create.go`**

Create `internal/network/routetable/route/create.go`:

```go
package route

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command, name, routeTableName, resourceGroup, addressPrefix, nextHopType, nextHopIP string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewRoutesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create routes client: %w", err)
	}

	props := &armnetwork.RoutePropertiesFormat{
		AddressPrefix: to.Ptr(addressPrefix),
		NextHopType:   to.Ptr(armnetwork.RouteNextHopType(nextHopType)),
	}
	if nextHopIP != "" {
		props.NextHopIPAddress = to.Ptr(nextHopIP)
	}

	parameters := armnetwork.Route{
		Properties: props,
	}

	fmt.Printf("Creating route '%s' in route table '%s'...\n", name, routeTableName)
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, routeTableName, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create route: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create route: %w", err)
	}

	return output.PrintJSON(cmd, result.Route)
}

// ValidateNextHopType returns nil if the provided next-hop-type is one of the
// allowed values; otherwise an error.
func ValidateNextHopType(v string) error {
	switch v {
	case "VirtualNetworkGateway", "VnetLocal", "Internet", "VirtualAppliance", "None":
		return nil
	}
	return fmt.Errorf("invalid --next-hop-type %q (must be one of: VirtualNetworkGateway, VnetLocal, Internet, VirtualAppliance, None)", v)
}
```

- [ ] **Step 2: Add `create` subcommand**

In `internal/network/routetable/route/commands.go`, before the final `cmd.AddCommand(...)`, add:

```go
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a route in a route table",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			routeTableName, _ := cmd.Flags().GetString("route-table-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			addressPrefix, _ := cmd.Flags().GetString("address-prefix")
			nextHopType, _ := cmd.Flags().GetString("next-hop-type")
			nextHopIP, _ := cmd.Flags().GetString("next-hop-ip-address")

			if err := ValidateNextHopType(nextHopType); err != nil {
				return err
			}
			if nextHopType == "VirtualAppliance" && nextHopIP == "" {
				return fmt.Errorf("--next-hop-ip-address is required when --next-hop-type is VirtualAppliance")
			}

			return Create(context.Background(), cmd, name, routeTableName, resourceGroup, addressPrefix, nextHopType, nextHopIP)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Route name")
	createCmd.Flags().String("route-table-name", "", "Route table name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().String("address-prefix", "", "Destination address prefix in CIDR format (e.g., 10.0.0.0/24)")
	createCmd.Flags().String("next-hop-type", "", "Next hop type: VirtualNetworkGateway, VnetLocal, Internet, VirtualAppliance, None")
	createCmd.Flags().String("next-hop-ip-address", "", "Next hop IP address (required when --next-hop-type is VirtualAppliance)")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("route-table-name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("address-prefix")
	createCmd.MarkFlagRequired("next-hop-type")
```

Make sure `fmt` is imported in `commands.go` (add to import block if not already there):

```go
import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)
```

Update AddCommand to: `cmd.AddCommand(listCmd, showCmd, createCmd)`.

- [ ] **Step 3: Build + commit**

```bash
make build
git add internal/network/routetable/route/create.go internal/network/routetable/route/commands.go
git commit -m "feat(route-table): add nested route create command"
```

---

### Task 13: Add `route delete`

**Files:**
- Create: `internal/network/routetable/route/delete.go`
- Modify: `internal/network/routetable/route/commands.go`

- [ ] **Step 1: Create `delete.go`**

Create `internal/network/routetable/route/delete.go`:

```go
package route

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Delete(ctx context.Context, name, routeTableName, resourceGroup string, noWait bool) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armnetwork.NewRoutesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create routes client: %w", err)
	}

	poller, err := client.BeginDelete(ctx, resourceGroup, routeTableName, name, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete route: %w", err)
	}

	if noWait {
		fmt.Printf("Started deletion of route '%s'\n", name)
		return nil
	}

	fmt.Printf("Deleting route '%s' from route table '%s'...\n", name, routeTableName)
	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to delete route: %w", err)
	}

	fmt.Printf("Deleted route '%s'\n", name)
	return nil
}
```

- [ ] **Step 2: Add `delete` subcommand**

In `internal/network/routetable/route/commands.go`, before the final `cmd.AddCommand(...)`, add:

```go
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a route from a route table",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			routeTableName, _ := cmd.Flags().GetString("route-table-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Delete(context.Background(), name, routeTableName, resourceGroup, noWait)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Route name")
	deleteCmd.Flags().String("route-table-name", "", "Route table name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("route-table-name")
	deleteCmd.MarkFlagRequired("resource-group")
```

Update AddCommand to: `cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)`.

- [ ] **Step 3: Build + commit**

```bash
make build
git add internal/network/routetable/route/delete.go internal/network/routetable/route/commands.go
git commit -m "feat(route-table): add nested route delete command"
```

---

### Task 14: Mount `route` under `route-table`

**Files:**
- Modify: `internal/network/routetable/commands.go`

- [ ] **Step 1: Add import**

In `internal/network/routetable/commands.go`, add to the import block:

```go
"github.com/cdobbyn/azure-go-cli/internal/network/routetable/route"
```

- [ ] **Step 2: Mount it**

Change the final AddCommand line from:

```go
cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd)
```

to:

```go
cmd.AddCommand(listCmd, showCmd, createCmd, deleteCmd, route.NewRouteCommand())
```

- [ ] **Step 3: Build**

Run: `make build`
Expected: success.

- [ ] **Step 4: Smoke**

Run: `./bin/az/az network route-table route --help`
Expected: shows `list`, `show`, `create`, `delete`.

- [ ] **Step 5: Commit**

```bash
git add internal/network/routetable/commands.go
git commit -m "feat(route-table): mount nested route subcommand"
```

---

### Task 15: End-to-end smoke verification

**Files:** none modified.

This task is verification only — exercise the new commands against a real subscription. Skip steps that require resources you don't have; record findings.

- [ ] **Step 1: Confirm path discovery**

```bash
./bin/az/az network vnet subnet --help
./bin/az/az network subnet --help
./bin/az/az network vnet subnet update --help
./bin/az/az network route-table --help
./bin/az/az network route-table route --help
```

Expected: each shows the documented subcommands and flags.

- [ ] **Step 2: Replay the original failing command**

Run (using a real RG + VNet you have access to):

```bash
./bin/az/az network vnet subnet list \
  --resource-group <rg> \
  --vnet-name <vnet> \
  --query "[].{name:name, prefix:addressPrefix, prefixes:addressPrefixes, nsg:networkSecurityGroup.id}" \
  -o json
```

Expected: JSON array, JMESPath projection applied (no `--query` errors), `prefixes` populated for VNets that use multi-prefix subnets.

- [ ] **Step 3: Old path still works**

```bash
./bin/az/az network subnet list -g <rg> --vnet-name <vnet>
```

Expected: same output as `network vnet subnet list`.

- [ ] **Step 4: `subnet update` smoke (if test resources available)**

```bash
./bin/az/az network vnet subnet update -n <subnet> --vnet-name <vnet> -g <rg> --network-security-group <nsg-name>
./bin/az/az network vnet subnet update -n <subnet> --vnet-name <vnet> -g <rg> --network-security-group ""
```

Expected: first call attaches the NSG (output shows `networkSecurityGroup.id`); second call detaches (NSG field gone or null).

- [ ] **Step 5: `route-table` smoke (if test resources available)**

```bash
./bin/az/az network route-table create -n smoke-rt -g <rg> -l eastus
./bin/az/az network route-table list -g <rg>
./bin/az/az network route-table route create -n smoke-route --route-table-name smoke-rt -g <rg> --address-prefix 10.99.0.0/24 --next-hop-type VirtualAppliance --next-hop-ip-address 10.99.1.4
./bin/az/az network route-table route list --route-table-name smoke-rt -g <rg>
./bin/az/az network route-table route delete -n smoke-route --route-table-name smoke-rt -g <rg>
./bin/az/az network route-table delete -n smoke-rt -g <rg>
```

Expected: each command succeeds. `route create` without `--next-hop-ip-address` while `--next-hop-type VirtualAppliance` should error before calling Azure.

- [ ] **Step 6: Final commit (if any docs touched during smoke)**

If smoke surfaced no code changes, no commit needed. If it did, commit them with `fix(...)` messages tied to the exact issue.

---

## Done definition

- `az network vnet subnet list ... --query "..." -o json` works against a real VNet and emits both `addressPrefix` and `addressPrefixes` when applicable.
- `az network subnet ...` (old path) still works.
- `az network vnet subnet update` can attach and detach NSG / route-table / NAT-gateway / service-endpoints / delegations.
- `az network route-table` and `az network route-table route` provide full CRUD.
- `make build` clean. No new lint warnings.
- All commits authored on branch `feat/vnet-subnet-and-route-table`.
