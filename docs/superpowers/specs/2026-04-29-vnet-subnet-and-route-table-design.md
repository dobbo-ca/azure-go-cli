# vnet subnet path + subnet update + route-table commands

**Date:** 2026-04-29
**Status:** Approved (design phase)
**Branch:** `feat/vnet-subnet-and-route-table`

## Background

Today, our CLI exposes subnet commands at `az network subnet ...`. The official Azure CLI uses `az network vnet subnet ...`. Users running scripts copied from Azure docs hit "command not found" or unrecognized path. Additionally:

- Existing `subnet/list.go` prints JSON with `fmt.Println(json.Marshal(...))`, bypassing the global `--query` (JMESPath) flag wired up in `pkg/output.PrintJSON`.
- `formatSubnet` returns `addressPrefix` (singular) but not `addressPrefixes` (plural) — the latter is the field newer Azure VNets populate.
- No `subnet update` command — users can't attach/detach NSG, route table, NAT gateway, service endpoints, or delegations without recreating.
- No `route-table` command group at all.

## Goals

1. `az network vnet subnet ...` works (matching official az CLI path).
2. `az network subnet ...` continues to work (no breaking change for existing scripts).
3. `--query` and `--output` flags work on every read command in scope.
4. `subnet update` supports attach/detach for NSG, route table, NAT gateway, service endpoints, delegations.
5. `route-table` group has full CRUD plus nested `route-table route` subgroup with full CRUD.

## Non-goals

- No backwards-compat aliasing beyond keeping the existing `network subnet` path.
- No automated tests (network packages currently have no tests; manual verification only).
- No new abstractions — extend existing patterns.

## Architecture

### File layout

```
internal/network/
├── commands.go              # mounts subnet under network AND under vnet; mounts routetable under network
├── subnet/
│   ├── commands.go          # NewSubnetCommand() factory — fresh tree per call
│   ├── list.go              # refactor to pkg/output.PrintJSON; add addressPrefixes
│   ├── show.go              # refactor to pkg/output.PrintJSON
│   ├── create.go            # unchanged
│   ├── delete.go            # unchanged
│   └── update.go            # NEW
├── routetable/              # NEW package
│   ├── commands.go          # NewRouteTableCommand() factory
│   ├── list.go
│   ├── show.go
│   ├── create.go
│   ├── delete.go
│   └── route/               # NEW nested package
│       ├── commands.go      # NewRouteCommand() factory
│       ├── list.go
│       ├── show.go
│       ├── create.go
│       └── delete.go
└── vnet/
    └── commands.go          # adds subnet.NewSubnetCommand() as a subcommand
```

### Dual-mount pattern

`subnet.NewSubnetCommand()` already returns a fresh `*cobra.Command` tree on every call. Mount under both parents:

```go
// internal/network/commands.go
networkCmd.AddCommand(
    subnet.NewSubnetCommand(),       // az network subnet ...
    routetable.NewRouteTableCommand(), // az network route-table ...
    // existing siblings…
)

// internal/network/vnet/commands.go
vnetCmd.AddCommand(
    listCmd, showCmd, createCmd, deleteCmd,
    subnet.NewSubnetCommand(),       // az network vnet subnet ...
)
```

Two independent cobra trees backed by the same package-level Run functions in `internal/network/subnet/*.go`. Negligible startup cost (factory is invoked twice).

### Output flow

All read commands (`list`, `show`) and mutating commands route their final output through `pkg/output.PrintJSON(cmd, data)`. That function reads the persistent `--query` flag from root and applies JMESPath; it then prints with respect to `-o`/`--output` (today only `json` is honored — the persistent flag accepts `json|table|tsv|yaml|none` but `PrintJSON` ignores it and always prints JSON; this is acceptable scope-wise as it matches existing commands).

## Component details

### `subnet update`

**Flags:**

| Flag | Purpose |
|------|---------|
| `--name` / `-n` (required) | subnet name |
| `--vnet-name` (required) | parent vnet |
| `--resource-group` / `-g` (required) | rg |
| `--network-security-group` | NSG resource ID or bare name; empty string clears |
| `--route-table` | route table resource ID or bare name; empty string clears |
| `--nat-gateway` | NAT gateway resource ID or bare name; empty string clears |
| `--service-endpoints` | comma-separated list (e.g., `Microsoft.Storage,Microsoft.KeyVault`); empty string clears |
| `--delegations` | comma-separated service names (e.g., `Microsoft.ContainerInstance/containerGroups`); empty string clears |
| `--no-wait` | skip waiting for LRO |

**Cobra flag-set semantics:** Use `cmd.Flags().Changed("flag")` to distinguish "not provided" from "provided as empty string." Empty string explicitly means clear.

**Resource ID resolution:**

For the three sub-resource flags (`--network-security-group`, `--route-table`, `--nat-gateway`):

- Value starting with `/subscriptions/` → use as-is.
- Bare name (no slash) → construct full ID using current default subscription + the same resource group as the subnet:
  - NSG: `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/networkSecurityGroups/{name}`
  - Route table: `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/routeTables/{name}`
  - NAT gateway: `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/natGateways/{name}`

**Implementation pattern:**

```go
func Update(ctx context.Context, cmd *cobra.Command, name, rg, vnet string, ...) error {
    // 1. Get current subnet
    current, err := client.Get(ctx, rg, vnet, name, nil)
    // 2. Mutate properties per flags (cmd.Flags().Changed checks)
    // 3. PUT via client.BeginCreateOrUpdate
    // 4. Poll unless --no-wait
    // 5. PrintJSON the result
}
```

### `route-table` group

**SDK clients:** `armnetwork.NewRouteTablesClient`, `armnetwork.NewRoutesClient`.

| Subcommand | Flags |
|------------|-------|
| `list` | `--resource-group` / `-g` (optional; lists all in subscription if omitted) |
| `show` | `--name` / `-n`, `--resource-group` / `-g` |
| `create` | `--name`, `--resource-group`, `--location` / `-l`, `--disable-bgp-route-propagation` (bool, default false), `--tags` (string-to-string) |
| `delete` | `--name`, `--resource-group`, `--no-wait` |

### `route-table route` nested group

| Subcommand | Flags |
|------------|-------|
| `list` | `--route-table-name`, `--resource-group` |
| `show` | `--name` / `-n`, `--route-table-name`, `--resource-group` |
| `create` | `--name`, `--route-table-name`, `--resource-group`, `--address-prefix` (CIDR), `--next-hop-type` (one of `VirtualNetworkGateway`, `VnetLocal`, `Internet`, `VirtualAppliance`, `None`), `--next-hop-ip-address` (required only when `next-hop-type` is `VirtualAppliance`) |
| `delete` | `--name`, `--route-table-name`, `--resource-group`, `--no-wait` |

Validate `--next-hop-type` against the allowed enum at the cobra layer (return error before calling SDK).

### `subnet/list.go` changes

```go
// Before:
data, _ := json.MarshalIndent(subnets, "", "  ")
fmt.Println(string(data))

// After:
return output.PrintJSON(cmd, subnets)
```

Pass `cmd *cobra.Command` through `List(ctx, cmd, vnetName, rg)`. Update call sites in `subnet/commands.go`.

### `formatSubnet` changes

Add `addressPrefixes` field when `subnet.Properties.AddressPrefixes` is non-nil/non-empty:

```go
if len(subnet.Properties.AddressPrefixes) > 0 {
    prefixes := make([]string, 0, len(subnet.Properties.AddressPrefixes))
    for _, p := range subnet.Properties.AddressPrefixes {
        if p != nil {
            prefixes = append(prefixes, *p)
        }
    }
    result["addressPrefixes"] = prefixes
}
```

Keep `addressPrefix` (singular) for backwards compatibility.

## Error handling

- All errors wrap with `fmt.Errorf("failed to <op>: %w", err)` — matches existing style in `subnet/list.go`.
- Cobra-level flag validation (e.g., `--next-hop-type` enum, mutually-exclusive future flags) returns error from `RunE` before SDK call.
- Resource-not-found surfaces the SDK's wrapped error verbatim.

## Manual verification

After implementation:

1. `./bin/az/az network vnet subnet list -g <rg> --vnet-name <vnet> --query "[].{name:name, prefix:addressPrefix, prefixes:addressPrefixes, nsg:networkSecurityGroup.id}" -o json` — original failing command from user request.
2. `./bin/az/az network subnet list -g <rg> --vnet-name <vnet>` — old path still works.
3. `./bin/az/az network vnet subnet update -n <s> --vnet-name <v> -g <rg> --network-security-group <nsg-id>` — attach NSG.
4. `./bin/az/az network vnet subnet update -n <s> --vnet-name <v> -g <rg> --network-security-group ""` — detach NSG.
5. `./bin/az/az network vnet subnet update ... --service-endpoints Microsoft.Storage,Microsoft.KeyVault` — set service endpoints.
6. `./bin/az/az network route-table list -g <rg>`.
7. `./bin/az/az network route-table create -n test-rt -g <rg> -l eastus`.
8. `./bin/az/az network route-table route create -n test-route --route-table-name test-rt -g <rg> --address-prefix 10.0.0.0/24 --next-hop-type VirtualAppliance --next-hop-ip-address 10.0.1.4`.
9. `./bin/az/az network route-table route list --route-table-name test-rt -g <rg>`.

## Out of scope

- `--output table/tsv/yaml` formats (existing limitation, separate work).
- Cross-subscription resource ID resolution for `subnet update` flags (assumes same sub as default).
- Subnet `update` flags for: address prefixes, private endpoint network policies, private link service network policies, IPAM pools.
- Route table peering routes / effective routes commands.
- Tests — all existing network packages lack tests; out of scope for this change.
