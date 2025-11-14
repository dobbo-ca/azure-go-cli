# Azure Go CLI - Implementation Tracking

## 📊 Quick Status (Updated 2025-11-13)

**Current Phase:** Core infrastructure complete ✅
**Next Phase:** VM infrastructure OR AKS write operations

### Completed This Session (2025-11-13)
- ✅ Storage blob operations (list, show, upload, download, delete)
  - Complete CRUD operations for blob storage
  - Upload/download with progress indicators
  - Table output for list, JSON for show
- ✅ AKS nodepool scale operation
  - Scale node pools up or down
  - Long-running operation with polling
  - Shows current count before scaling

### Previously Completed (2025-11-11)
- ✅ All 14 priority resource types have list/show commands
- ✅ 26 commands implemented (13 list + 13 show)
- ✅ Added Managed Identity support (az identity list/show)
- ✅ All commands tested against live Azure environment
- ✅ SDK upgraded to v6 for AKS and Network resources
- ✅ AKS Bastion enhancements:
  - Per-connection WebSocket token exchange (fixes timeout issues)
  - `--cmd` flag for running arbitrary commands with KUBECONFIG
  - Automatic clipboard integration for export commands
  - Device code authentication with browser opening

### Previously Completed (2025-11-12)
- ✅ VM infrastructure commands (public-ip, nic, disk, vm create)
  - Complete VM lifecycle management
  - Disk attachment and management
  - Network interface configuration

### Ready to Resume
See [Next Priorities](#-next-priorities) section below for recommended next steps.

### File Structure Quick Reference
```
cdobbyn/azure-go-cli/
├── cmd/az/main.go              # Main entry point, register all commands here
├── internal/
│   ├── aks/                    # AKS commands (cluster, nodepool, addon, etc.)
│   ├── network/                # Network commands (vnet, subnet, nat, lb, etc.)
│   │   └── commands.go         # Register network subcommands here
│   ├── storage/                # Storage commands (account, container)
│   ├── postgres/               # PostgreSQL commands (flexible-server)
│   ├── keyvault/               # Key Vault commands
│   ├── identity/               # Managed Identity commands
│   └── group/                  # Resource group commands
├── pkg/
│   ├── azure/                  # Azure helpers (credentials, string utils)
│   └── config/                 # Config management (subscription)
└── TODO.md                     # This file
```

---

## Current Status

This document tracks the implementation status of the Azure Go CLI, focusing on our priority resource types.

## Priority Focus Areas

We are prioritizing implementation for the following Azure resource types:

1. Azure Kubernetes Service (AKS)
2. Virtual Networks
3. NAT Gateway
4. VPN Gateway
5. Load Balancers
6. Flexible Server for PostgreSQL
7. Key Vault
8. Resource Groups
9. Storage Accounts
10. Blob Storage
11. Subnets and other networking components
12. VNet Peering and routing
13. Private Endpoints
14. Managed Identities

---

## Implementation Status

### Authentication & Account Management
- [x] `az login` - Device code authentication
- [x] `az logout` / `az logoff` - Sign out
- [x] `az account list` - List subscriptions
- [x] `az account show` - Show current subscription
- [x] `az account set` - Set default subscription

### Azure Kubernetes Service (AKS)

#### Core Cluster Commands
- [x] `az aks list` - List AKS clusters
- [x] `az aks show` - Show cluster details
- [x] `az aks get-credentials` - Get kubeconfig
- [x] `az aks bastion` - Connect via Azure Bastion with enhancements:
  - Per-connection WebSocket token exchange for stable multi-command usage
  - `--cmd` flag to run arbitrary commands (e.g., `--cmd "kubectl get pods"` or `--cmd "k9s"`)
  - Automatic clipboard integration for export commands
  - Device code authentication with browser opening
- [ ] `az aks get-versions` - Get available K8s versions
- [ ] `az aks get-upgrades` - Get available cluster upgrades

#### Node Pool Management (`az aks nodepool`)
- [x] `az aks nodepool list` - List node pools in cluster
- [x] `az aks nodepool show` - Show node pool details
- [x] `az aks nodepool scale` - Scale node pool to target count
- [ ] `az aks nodepool add` - Add node pool (write operation)
- [ ] `az aks nodepool delete` - Delete node pool (write operation)
- [ ] `az aks nodepool update` - Update node pool (write operation)
- [ ] `az aks nodepool upgrade` - Upgrade node pool (write operation)
- [ ] `az aks nodepool get-upgrades` - Get available node pool upgrades

#### Addon Management (`az aks addon`)
- [x] `az aks addon list` - List cluster addons
- [x] `az aks addon list-available` - List available addons
- [x] `az aks addon show` - Show addon details
- [ ] `az aks addon enable` - Enable addon (write operation)
- [ ] `az aks addon disable` - Disable addon (write operation)
- [ ] `az aks addon update` - Update addon (write operation)

#### Machine Management (`az aks machine`)
- [x] `az aks machine list` - List machines in node pool
- [x] `az aks machine show` - Show machine details

#### Maintenance Configuration (`az aks maintenanceconfiguration`)
- [x] `az aks maintenanceconfiguration list` - List maintenance configs
- [x] `az aks maintenanceconfiguration show` - Show maintenance config details
- [ ] `az aks maintenanceconfiguration add` - Add maintenance config (write operation)
- [ ] `az aks maintenanceconfiguration delete` - Delete maintenance config (write operation)
- [ ] `az aks maintenanceconfiguration update` - Update maintenance config (write operation)

#### Cluster Snapshots (`az aks snapshot`)
- [x] `az aks snapshot list` - List cluster snapshots
- [x] `az aks snapshot show` - Show snapshot details
- [ ] `az aks snapshot create` - Create snapshot (write operation)
- [ ] `az aks snapshot delete` - Delete snapshot (write operation)

#### Operations (`az aks operation`)
- [x] `az aks operation show` - Show operation details (limited - SDK v6 doesn't support operation ID lookup)
- [x] `az aks operation show-latest` - Show latest operation

#### Pod Identity (`az aks pod-identity`)
- [x] `az aks pod-identity list` - List pod identities
- [ ] `az aks pod-identity add` - Add pod identity (write operation)
- [ ] `az aks pod-identity delete` - Delete pod identity (write operation)

#### Other Write Operations (Future)
- [ ] `az aks create` - Create cluster
- [ ] `az aks delete` - Delete cluster
- [ ] `az aks update` - Update cluster
- [ ] `az aks upgrade` - Upgrade cluster
- [ ] `az aks scale` - Scale cluster
- [ ] `az aks start` - Start stopped cluster
- [ ] `az aks stop` - Stop cluster
- [ ] `az aks rotate-certs` - Rotate certificates
- [ ] `az aks update-credentials` - Update credentials
- [ ] `az aks operation-abort` - Abort running operation

### Resource Groups

- [x] `az group list` - List resource groups
- [x] `az group show` - Show resource group details
- [ ] `az group create` - Create resource group (write operation)
- [ ] `az group delete` - Delete resource group (write operation)
- [ ] `az group update` - Update resource group (write operation)
- [ ] `az group exists` - Check if resource group exists

### Network - Virtual Networks

#### Virtual Network (`az network vnet`)
- [x] `az network vnet list` - List virtual networks
- [x] `az network vnet show` - Show VNet details
- [ ] `az network vnet create` - Create VNet (write operation)
- [ ] `az network vnet delete` - Delete VNet (write operation)
- [ ] `az network vnet update` - Update VNet (write operation)

#### Subnets (`az network subnet`)
- [x] `az network subnet list` - List subnets
- [x] `az network subnet show` - Show subnet details
- [ ] `az network subnet create` - Create subnet (write operation)
- [ ] `az network subnet delete` - Delete subnet (write operation)
- [ ] `az network subnet update` - Update subnet (write operation)

#### VNet Peering (`az network peering`)
- [x] `az network peering list` - List VNet peerings
- [x] `az network peering show` - Show peering details
- [ ] `az network peering create` - Create peering (write operation)
- [ ] `az network peering delete` - Delete peering (write operation)
- [ ] `az network peering update` - Update peering (write operation)

### Network - Gateways

#### NAT Gateway (`az network nat`)
- [x] `az network nat list` - List NAT gateways
- [x] `az network nat show` - Show NAT gateway details
- [ ] `az network nat create` - Create NAT gateway (write operation)
- [ ] `az network nat delete` - Delete NAT gateway (write operation)
- [ ] `az network nat update` - Update NAT gateway (write operation)

#### VPN Gateway (`az network vnet-gateway`)
- [x] `az network vnet-gateway list` - List VPN gateways (requires resource group)
- [x] `az network vnet-gateway show` - Show VPN gateway details
- [ ] `az network vnet-gateway create` - Create VPN gateway (write operation)
- [ ] `az network vnet-gateway delete` - Delete VPN gateway (write operation)
- [ ] `az network vnet-gateway update` - Update VPN gateway (write operation)
- [ ] `az network vnet-gateway list-bgp-peer-status` - List BGP peer status
- [ ] `az network vnet-gateway list-learned-routes` - List learned routes
- [ ] `az network vnet-gateway list-advertised-routes` - List advertised routes

### Network - Load Balancing

#### Load Balancer (`az network lb`)
- [x] `az network lb list` - List load balancers
- [x] `az network lb show` - Show load balancer details
- [ ] `az network lb create` - Create load balancer (write operation)
- [ ] `az network lb delete` - Delete load balancer (write operation)
- [ ] `az network lb update` - Update load balancer (write operation)

#### Load Balancer Frontend IP (`az network lb frontend-ip`)
- [ ] `az network lb frontend-ip list` - List frontend IPs
- [ ] `az network lb frontend-ip show` - Show frontend IP details

#### Load Balancer Backend Pool (`az network lb address-pool`)
- [ ] `az network lb address-pool list` - List backend pools
- [ ] `az network lb address-pool show` - Show backend pool details

#### Load Balancer Rules (`az network lb rule`)
- [ ] `az network lb rule list` - List load balancing rules
- [ ] `az network lb rule show` - Show rule details

#### Load Balancer Probes (`az network lb probe`)
- [ ] `az network lb probe list` - List health probes
- [ ] `az network lb probe show` - Show probe details

### Network - Private Connectivity

#### Private Endpoint (`az network private-endpoint`)
- [x] `az network private-endpoint list` - List private endpoints
- [x] `az network private-endpoint show` - Show private endpoint details
- [ ] `az network private-endpoint create` - Create private endpoint (write operation)
- [ ] `az network private-endpoint delete` - Delete private endpoint (write operation)
- [ ] `az network private-endpoint update` - Update private endpoint (write operation)

#### Private Link Service (`az network private-link-service`)
- [ ] `az network private-link-service list` - List private link services
- [ ] `az network private-link-service show` - Show private link service details

#### Bastion (Already Implemented)
- [x] `az network bastion tunnel` - Create SSH/RDP tunnel via Bastion

### Database - PostgreSQL Flexible Server

#### Server (`az postgres flexible-server`)
- [x] `az postgres flexible-server list` - List PostgreSQL servers
- [x] `az postgres flexible-server show` - Show server details
- [ ] `az postgres flexible-server create` - Create server (write operation)
- [ ] `az postgres flexible-server delete` - Delete server (write operation)
- [ ] `az postgres flexible-server update` - Update server (write operation)
- [ ] `az postgres flexible-server start` - Start server (write operation)
- [ ] `az postgres flexible-server stop` - Stop server (write operation)
- [ ] `az postgres flexible-server restart` - Restart server (write operation)

#### Database (`az postgres flexible-server db`)
- [ ] `az postgres flexible-server db list` - List databases
- [ ] `az postgres flexible-server db show` - Show database details
- [ ] `az postgres flexible-server db create` - Create database (write operation)
- [ ] `az postgres flexible-server db delete` - Delete database (write operation)

#### Firewall Rules (`az postgres flexible-server firewall-rule`)
- [ ] `az postgres flexible-server firewall-rule list` - List firewall rules
- [ ] `az postgres flexible-server firewall-rule show` - Show firewall rule details

#### Parameters (`az postgres flexible-server parameter`)
- [ ] `az postgres flexible-server parameter list` - List server parameters
- [ ] `az postgres flexible-server parameter show` - Show parameter details

### Security - Key Vault

#### Vault (`az keyvault`)
- [x] `az keyvault list` - List key vaults
- [x] `az keyvault show` - Show key vault details
- [ ] `az keyvault create` - Create key vault (write operation)
- [ ] `az keyvault delete` - Delete key vault (write operation)
- [ ] `az keyvault update` - Update key vault (write operation)
- [ ] `az keyvault purge` - Purge deleted key vault (write operation)
- [ ] `az keyvault list-deleted` - List deleted key vaults

#### Secrets (`az keyvault secret`)
- [ ] `az keyvault secret list` - List secrets
- [ ] `az keyvault secret show` - Show secret (without value)
- [ ] `az keyvault secret set` - Set secret value (write operation)
- [ ] `az keyvault secret delete` - Delete secret (write operation)

#### Keys (`az keyvault key`)
- [ ] `az keyvault key list` - List keys
- [ ] `az keyvault key show` - Show key details

#### Certificates (`az keyvault certificate`)
- [ ] `az keyvault certificate list` - List certificates
- [ ] `az keyvault certificate show` - Show certificate details

### Identity - Managed Identities

#### Managed Identity (`az identity`)
- [x] `az identity list` - List managed identities
- [x] `az identity show` - Show managed identity details
- [ ] `az identity create` - Create managed identity (write operation)
- [ ] `az identity delete` - Delete managed identity (write operation)
- [ ] `az identity list-operations` - List available operations

### Storage - Accounts

#### Storage Account (`az storage account`)
- [x] `az storage account list` - List storage accounts
- [x] `az storage account show` - Show storage account details
- [ ] `az storage account create` - Create storage account (write operation)
- [ ] `az storage account delete` - Delete storage account (write operation)
- [ ] `az storage account update` - Update storage account (write operation)
- [ ] `az storage account show-connection-string` - Show connection string
- [ ] `az storage account keys list` - List account keys

#### Blob Storage (`az storage container`)
- [x] `az storage container list` - List blob containers (requires account name and resource group)
- [x] `az storage container show` - Show container details
- [ ] `az storage container create` - Create container (write operation)
- [ ] `az storage container delete` - Delete container (write operation)

#### Blob Operations (`az storage blob`)
- [x] `az storage blob list` - List blobs (requires account name and container name)
- [x] `az storage blob show` - Show blob details (JSON output)
- [x] `az storage blob upload` - Upload blob to storage
- [x] `az storage blob download` - Download blob to local file
- [x] `az storage blob delete` - Delete blob from storage

---

## Unimplemented Services

The following Azure services are **not currently planned** for implementation. Commands for these services will not work:

- `az vm` - Virtual Machines
- `az vmss` - Virtual Machine Scale Sets
- `az container` - Container Instances
- `az functionapp` - Function Apps
- `az webapp` - Web Apps
- `az appservice` - App Service
- `az cosmosdb` - Cosmos DB
- `az sql` - SQL Database
- `az mysql` - MySQL
- `az redis` - Redis Cache
- `az monitor` - Azure Monitor
- `az log-analytics` - Log Analytics
- `az application-insights` - Application Insights
- `az disk` - Managed Disks
- `az snapshot` - Disk Snapshots
- `az image` - VM Images
- `az acr` - Container Registry
- `az ad` - Azure Active Directory
- `az role` - Role-Based Access Control
- `az policy` - Azure Policy
- `az blueprint` - Azure Blueprints
- `az backup` - Azure Backup
- `az recovery-services` - Recovery Services
- `az cdn` - Content Delivery Network
- `az dns` - DNS
- `az traffic-manager` - Traffic Manager
- `az front-door` - Front Door
- `az firewall` - Azure Firewall
- `az application-gateway` - Application Gateway
- `az route-table` - Route Tables
- `az route-filter` - Route Filters
- `az nsg` - Network Security Groups (NSG)
- `az network-watcher` - Network Watcher
- `az express-route` - ExpressRoute
- `az eventhub` - Event Hubs
- `az servicebus` - Service Bus
- `az iot` - IoT Hub
- `az synapse` - Azure Synapse
- `az databricks` - Azure Databricks
- `az hdinsight` - HDInsight
- `az batch` - Batch
- And many more...

---

## Current Implementation Status (2025-10-15)

### ✅ Completed
All **list** and **show** commands for the 14 priority resource types have been implemented and tested:
- AKS (cluster, nodepool, addon, machine, maintenance config, snapshot, operation, pod-identity)
- Resource Groups (list, show)
- Virtual Networks (vnet, subnet, peering)
- Network Gateways (NAT, VPN)
- Load Balancers
- Private Endpoints
- Storage (accounts, containers)
- PostgreSQL Flexible Server
- Key Vault
- Managed Identities (list, show)

All commands tested successfully against live Azure environment with proper JSON output.

### 🎯 Next Priorities

When resuming work, the recommended priorities are:

1. **VM Infrastructure Expansion** (quick wins, ~30-45 min each)
   - Option A: VM lifecycle operations (start, stop, restart, deallocate)
   - Option B: VM network operations (attach/detach NIC, update IP config)
   - Option C: VM disk operations (attach/detach additional disks)
   - Option D: VM list/show operations (complete read operations)

2. **AKS Write Operations** (high business value, ~45-60 min each)
   - Start with AKS nodepool scale (most commonly needed)
   - Then AKS cluster upgrade/update operations
   - AKS addon enable/disable
   - AKS nodepool add/delete

3. **Storage Write Operations** (medium priority, ~20-30 min each)
   - Storage account create/delete
   - Storage container create/delete
   - Resource group create/delete

4. **Add Tests** (important for stability)
   - Unit tests for command parsing and formatting
   - Integration tests with mocked Azure SDK
   - Test fixtures for common responses

5. **Code Cleanup** (nice to have)
   - Extract duplicate `getResourceGroupFromID()` helper to pkg/azure/utils.go
   - Standardize error messages across all commands
   - Add consistent logging

6. **Additional Read Commands** (lower priority)
   - Load balancer subresources (frontend-ip, backend-pool, rule, probe)
   - PostgreSQL databases, firewall rules, parameters
   - Key Vault secrets, keys, certificates (read-only)

---

## Future Work

### Testing
- [ ] Add unit tests for all commands
- [ ] Add integration tests for all commands
- [ ] Add mocking framework for Azure SDK calls
- [ ] Add test fixtures and sample data
- [ ] Add CI/CD pipeline for automated testing

### Write Operations
Currently, we are focusing on **read-only operations** (list, show). Future work includes implementing:

**High Priority Write Operations:**
- [ ] `az aks nodepool scale` - Scale AKS node pools (most commonly needed)
- [ ] `az aks upgrade` - Upgrade AKS cluster K8s version
- [ ] `az aks update` - Update AKS cluster configuration
- [ ] `az aks nodepool upgrade` - Upgrade node pool K8s version
- [ ] `az group create` - Create resource groups
- [ ] `az group delete` - Delete resource groups
- [ ] `az storage container create` - Create blob containers
- [ ] `az storage container delete` - Delete blob containers

**Medium Priority Write Operations:**
- [ ] `az aks nodepool add` - Add new node pool
- [ ] `az aks nodepool delete` - Remove node pool
- [ ] `az aks addon enable` - Enable AKS addons
- [ ] `az aks addon disable` - Disable AKS addons
- [ ] `az storage account create` - Create storage accounts
- [ ] `az keyvault secret set` - Set Key Vault secrets
- [ ] `az postgres flexible-server restart` - Restart database

**Lower Priority Write Operations:**
- [ ] `az aks create` - Create new AKS cluster
- [ ] `az aks delete` - Delete AKS cluster
- [ ] `az network vnet create` - Create VNet
- [ ] `az network subnet create` - Create subnet
- [ ] All other create/update/delete operations listed above

### Documentation
- [ ] Add command examples to README
- [ ] Add usage documentation for each service
- [ ] Add troubleshooting guide
- [ ] Add contribution guidelines

### Developer Experience
- [ ] Add command completion/suggestions
- [ ] Add output formatting options (table, json, yaml)
- [ ] Add filtering and query capabilities
- [ ] Add verbose/debug logging options
- [ ] Add progress indicators for long-running operations

### Authentication Enhancements
- [ ] Keychain support with better UX (LOW priority)
  - Pre-authorize CLI in keychain to avoid repeated prompts
  - Add command-line flag: `--use-keychain` vs `--use-file-cache`
  - Provide clear setup instructions for keychain authorization
  - Consider codesigning the binary to reduce keychain prompts
- [ ] Token refresh testing (MEDIUM priority)
  - Test expired token refresh
  - Verify refresh tokens are used instead of re-prompting
  - Test multi-tenant scenarios
- [ ] Error message improvements (LOW priority)
  - "Not authenticated" should suggest `az login`
  - Authorization failures should check if user has required role
  - Network errors should be more descriptive

---

## Azure SDK Dependencies

Current dependencies:
```
github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6
github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources
github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions
github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6
github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage
github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers/v4
github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault
github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi
```

---

## Notes

- This CLI is designed as a **lightweight, focused alternative** to the official Azure CLI
- We prioritize **simplicity and maintainability** over feature completeness
- Commands follow the same patterns and flags as the official Azure CLI where possible
- All commands use the same authentication mechanism (device code flow)
- JSON output format is used by default for consistency and parseability

---

## Implementation Checklist

When adding new commands, follow this checklist:

### Adding a New List/Show Command
1. [ ] Create `internal/{service}/{resource}/` directory
2. [ ] Create `list.go` with List() function and formatX() helper
3. [ ] Create `show.go` with Show() function
4. [ ] Create `commands.go` with NewXCommand() and cobra setup
5. [ ] Register command in parent `commands.go` or `cmd/az/main.go`
6. [ ] Build: `go build -o az ./cmd/az`
7. [ ] Test list command: `./az {service} {resource} list`
8. [ ] Test show command: `./az {service} {resource} show -n {name} -g {rg}`
9. [ ] Update TODO.md checkboxes

### Adding a Write Operation (Create/Update/Delete)
1. [ ] Follow list/show pattern above for file structure
2. [ ] Create operation file (e.g., `scale.go`, `create.go`)
3. [ ] Add confirmation prompt for destructive operations (delete)
4. [ ] Use poller pattern for long-running operations (LRO)
5. [ ] Handle errors gracefully with clear messages
6. [ ] Test against non-production environment first!
7. [ ] Update TODO.md checkboxes

### Common Patterns
```go
// List pattern with pager
pager := client.NewListPager(resourceGroup, nil)
for pager.More() {
    page, err := pager.NextPage(ctx)
    // process page.Value
}

// Show pattern
result, err := client.Get(ctx, resourceGroup, name, nil)

// Create/Update pattern (LRO)
poller, err := client.BeginCreate(ctx, resourceGroup, name, params, nil)
result, err := poller.PollUntilDone(ctx, nil)

// Delete pattern with confirmation
fmt.Printf("Are you sure you want to delete %s? (yes/no): ", name)
// read confirmation, then delete
```
