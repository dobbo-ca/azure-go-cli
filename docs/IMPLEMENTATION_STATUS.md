# Azure Go CLI Implementation Status

This document tracks the implementation status of azure-go-cli compared to the official Azure CLI.

Last updated: 2025-11-13

## Overview

The azure-go-cli is a lightweight Go implementation of the Azure CLI focused on the most commonly used commands for infrastructure management and development workflows.

## Design Philosophy

- **Core Commands First**: Implement frequently-used commands before comprehensive coverage
- **Quality Over Quantity**: Each command should match official CLI behavior
- **Performance**: Go compilation provides fast startup times
- **Simplicity**: Minimal dependencies, easy to build and distribute

## Currently Implemented Commands

### Authentication & Account Management
- ✅ `az login` - Interactive and device code authentication
- ✅ `az logout` - Clear authentication state
- ✅ `az account list` - List subscriptions
- ✅ `az account show` - Show current subscription
- ✅ `az account set` - Set active subscription

### Resource Groups
- ✅ `az group list` - List resource groups
- ✅ `az group show` - Show resource group details
- ✅ `az group create` - Create resource group
- ✅ `az group delete` - Delete resource group

### Azure Kubernetes Service (AKS)
- ✅ `az aks list` - List AKS clusters
- ✅ `az aks show` - Show AKS cluster details
- ✅ `az aks get-credentials` - Get kubeconfig for cluster

### Managed Identities
- ✅ `az identity list` - List managed identities
- ✅ `az identity show` - Show identity details (supports --ids flag)

### Role-Based Access Control (RBAC)
- ✅ `az role list` - List role definitions
- ✅ `az role show` - Show role definition details
- ✅ `az role assignment list` - List role assignments
- ✅ `az role assignment create` - Create role assignment
- ✅ `az role assignment delete` - Delete role assignment

### Networking
- ✅ `az network vnet list` - List virtual networks
- ✅ `az network vnet show` - Show VNet details
- ✅ `az network vnet create` - Create VNet
- ✅ `az network vnet subnet list` - List subnets
- ✅ `az network vnet subnet show` - Show subnet details
- ✅ `az network nsg list` - List network security groups
- ✅ `az network nsg show` - Show NSG details
- ✅ `az network nsg rule list` - List NSG rules
- ✅ `az network nsg rule show` - Show NSG rule details
- ✅ `az network nsg rule create` - Create NSG rule
- ✅ `az network nsg rule delete` - Delete NSG rule
- ✅ `az network bastion tunnel` - Create tunnel through Azure Bastion

### Storage
- ✅ `az storage account list` - List storage accounts
- ✅ `az storage account show` - Show storage account details
- ✅ `az storage account keys list` - List storage account keys
- ✅ `az storage container list` - List blob containers
- ✅ `az storage blob list` - List blobs
- ✅ `az storage blob download` - Download blob

### PostgreSQL
- ✅ `az postgres server list` - List PostgreSQL servers
- ✅ `az postgres server show` - Show server details
- ✅ `az postgres flexible-server list` - List flexible servers
- ✅ `az postgres flexible-server show` - Show flexible server details

### Key Vault
- ✅ `az keyvault list` - List key vaults
- ✅ `az keyvault show` - Show key vault details
- ✅ `az keyvault secret list` - List secrets
- ✅ `az keyvault secret show` - Show secret value

### Virtual Machines
- ✅ `az vm list` - List virtual machines
- ✅ `az vm show` - Show VM details

### Quotas
- ✅ `az quota list` - List quota limits
- ✅ `az quota show` - Show quota details
- ✅ `az quota request list` - List quota requests

## Global Features

- ✅ `--subscription` flag - Override default subscription
- ✅ `--output` flag - Output format (json, table, tsv, yaml, none)
- ✅ `--query` flag - JMESPath query to filter output
- ✅ `--debug` flag - Enable debug logging

## Fallback to Official CLI

For commands not yet implemented, use the provided helper script:

```bash
# Use official CLI via container
./scripts/az-official <command> [args...]

# Example
./scripts/az-official network vpn-gateway list
```

## Implementation Priorities

Based on usage patterns in infrastructure automation:

### High Priority (Next)
- `az network vnet-gateway` - VPN gateway management
- `az network private-endpoint` - Private endpoint management
- `az monitor` - Metrics and alerting
- `az disk` - Managed disk operations

### Medium Priority
- `az container` - Azure Container Instances
- `az cosmosdb` - Cosmos DB management
- `az sql` - SQL Database management
- `az ad` - Azure AD operations

### Lower Priority
- `az iot` - IoT Hub management
- `az batch` - Batch job management
- Less frequently used service-specific commands

## Full Command Audit

For a complete list of all official Azure CLI commands and their implementation status, see:

```bash
./scripts/audit-commands.sh    # Generate audit report
cat docs/command-tree.md       # View command tree with status
```

## Contributing

When implementing new commands:

1. Check official CLI behavior: `./scripts/az-official <command> --help`
2. Implement using Azure SDK for Go
3. Add tests
4. Update this document
5. Re-run audit: `./scripts/audit-commands.sh`

## Testing Against Official CLI

To verify behavior matches official CLI:

```bash
# Official CLI
./scripts/az-official identity list

# Our implementation
./bin/az/az identity list

# Compare outputs
diff <(./scripts/az-official identity list | jq -S .) <(./bin/az/az identity list | jq -S .)
```
