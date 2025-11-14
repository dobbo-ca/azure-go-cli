# Azure Go CLI

A fast, lightweight Azure CLI implementation in Go with 100+ commands for managing Azure resources.

This project provides a performant alternative to the official Azure CLI, written in Go using the Azure SDK. It offers comprehensive resource management capabilities across compute, network, storage, database, and identity services.

## Features

### Authentication & Account Management
- `az login` - Authenticate with Azure using device code flow
- `az logout` - Sign out from Azure
- `az account` - Manage subscriptions and authentication tokens

### Compute
- `az vm` - Manage virtual machines (list, show, start, stop, delete, list-skus)

### Networking
- `az network vnet` - Manage virtual networks (CRUD operations)
- `az network subnet` - Manage subnets (CRUD operations)
- `az network nsg` - Manage network security groups (CRUD operations)
- `az network nsg rule` - Manage security rules (CRUD operations)
- `az network nat` - Manage NAT gateways (CRUD operations)
- `az network lb` - Manage load balancers (CRUD operations)
- `az network vnet-peering` - Manage VNet peering (CRUD operations)
- `az network private-endpoint` - Manage private endpoints (CRUD operations)
- `az network vnet-gateway` - Manage VPN gateways (CRUD operations)
- `az network bastion` - SSH and tunnel through Azure Bastion

### Storage
- `az storage account` - Manage storage accounts (CRUD operations)
- `az storage container` - Manage blob containers (CRUD operations)

### Databases
- `az postgres flexible-server` - Manage PostgreSQL flexible servers (CRUD operations)

### Identity & Access
- `az identity` - Manage managed identities (CRUD operations)
- `az role` - Manage role definitions and assignments
- `az group` - Manage resource groups (CRUD operations)

### Key Vault
- `az keyvault` - Manage Key Vaults (list, show)
- `az keyvault secret` - Manage secrets (list, show, set, delete)

### Kubernetes
- `az aks` - Manage Azure Kubernetes Service clusters
- `az aks nodepool` - Manage AKS node pools
- `az aks addon` - Manage AKS add-ons

### Quotas
- `az quota` - Manage and request service quotas

## Installation

### From Homebrew (macOS/Linux)

```bash
brew tap dobbo-ca/azure-go-cli
brew install azure-go-cli
```

### From Release Binaries

Download the latest release for your platform from the [releases page](https://github.com/dobbo-ca/azure-go-cli/releases).

### From Source

```bash
git clone https://github.com/dobbo-ca/azure-go-cli.git
cd azure-go-cli
make build
```

The binary will be created at `bin/az/az`.

## Usage

### Authentication

```bash
# Login to Azure
az login

# Set default subscription
az account set --subscription "My Subscription"

# Show current account
az account show
```

### Virtual Machines

```bash
# List all VMs
az vm list

# Show specific VM
az vm show --name my-vm --resource-group my-rg

# Start/stop VM
az vm start --name my-vm --resource-group my-rg
az vm stop --name my-vm --resource-group my-rg

# List available VM SKUs
az vm list-skus --location eastus
```

### Network Security

```bash
# Create NSG
az network nsg create --name my-nsg --resource-group my-rg --location eastus

# Create security rule
az network nsg rule create --name allow-ssh \
  --nsg-name my-nsg --resource-group my-rg \
  --priority 1000 --direction Inbound --access Allow \
  --protocol TCP --source-address-prefix "*" \
  --source-port-range "*" --destination-address-prefix "*" \
  --destination-port-range 22

# List rules
az network nsg rule list --nsg-name my-nsg --resource-group my-rg
```

### Storage

```bash
# Create storage account
az storage account create --name mystorageacct \
  --resource-group my-rg --location eastus

# Create blob container
az storage container create --name my-container \
  --account-name mystorageacct --resource-group my-rg
```

### Key Vault Secrets

```bash
# List secrets
az keyvault secret list --vault-name my-vault

# Show secret (without value)
az keyvault secret show --vault-name my-vault --name my-secret

# Show secret with value
az keyvault secret show --vault-name my-vault --name my-secret --show-value

# Set secret
az keyvault secret set --vault-name my-vault --name my-secret --value "secret-value"
```

### Output Formats

All commands support multiple output formats:

```bash
# JSON (default)
az vm list

# Table format
az vm list --output table

# YAML format
az vm list --output yaml

# TSV for scripting
az vm list --output tsv
```

### JMESPath Queries

Filter output using JMESPath:

```bash
# Get only VM names
az vm list --query "[].name"

# Filter by location
az vm list --query "[?location=='eastus']"
```

## Command Reference

For detailed command documentation, see:
- Full command tree: `docs/command-tree.md`
- Implementation status: Run `./scripts/audit-commands.sh`

## Development

### Building

```bash
make build        # Build for current platform
make all          # Build for all platforms
make test         # Run tests
make clean        # Clean build artifacts
```

### Project Structure

```
azure-go-cli/
├── cmd/az/              # Main entry point
├── internal/            # Internal packages
│   ├── account/        # Account management
│   ├── aks/            # AKS commands
│   ├── auth/           # Authentication
│   ├── group/          # Resource groups
│   ├── identity/       # Managed identities
│   ├── keyvault/       # Key Vault
│   ├── network/        # Network resources
│   ├── postgres/       # PostgreSQL
│   ├── storage/        # Storage
│   └── vm/             # Virtual machines
├── pkg/                # Public packages
│   ├── azure/          # Azure credential helpers
│   └── config/         # Configuration management
└── docs/               # Documentation
```

## Versioning & Releases

This project uses:
- **Conventional Commits** for commit messages
- **Semantic Versioning** for releases
- **Automated releases** via GitHub Actions with [Uplift](https://upliftci.dev)

Commit prefixes:
- `feat:` - New feature (minor version bump)
- `fix:` - Bug fix (patch version bump)
- `feat!:` or `BREAKING CHANGE:` - Breaking change (major version bump)

## Differences from Official Azure CLI

- **Performance**: Written in Go, faster startup and execution
- **Size**: Smaller binary (~50MB vs 1GB+ for Python-based CLI)
- **Subset of commands**: Focuses on most commonly used resource management operations
- **Compatible**: Uses same Azure SDK and authentication as official CLI

## Requirements

- Go 1.21 or later (for building from source)
- Azure subscription

## License

MIT License - see LICENSE file for details

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

See `CLAUDE.md` for development guidelines.
