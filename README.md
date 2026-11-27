# Azure Go CLI

A fast, lightweight Azure CLI implementation in Go with 100+ commands for managing Azure resources.

This project provides a performant alternative to the official Azure CLI, written in Go using the Azure SDK. It offers comprehensive resource management capabilities across compute, network, storage, database, and identity services.

## Capabilities beyond the Python Azure CLI

These features are not in the official Python Azure CLI, or work differently there. The [user guide](docs/user-guide.md) explains how to use each one, and [`docs/design/`](docs/design/) explains how each one is built.

- **Parallel sign-ins with AZ_SESSION** — keep several terminals signed in to different tenants or accounts, each with its own profile and token cache (default sign-in mode), carried into the kubeconfig written by `az aks bastion`. [Guide](docs/user-guide.md#parallel-sign-ins-with-az_session)
- **Tenant-first login** — `--tenant-selection` or more than 10 subscriptions asks for a tenant, then a subscription, and MFA tenants sign in inline instead of being skipped. [Guide](docs/user-guide.md#tenant-first-login)
- **Reusing the Python CLI sign-in (--use-azure-cli)** — borrow tokens from the Python `az` to satisfy device-based Conditional Access. [Guide](docs/user-guide.md#reusing-the-python-cli-sign-in---use-azure-cli)
- **Windows WAM broker** — WAM is used by default like Python, but this CLI bundles the broker DLL, lets you override it, retries in the browser if WAM fails, and `--broker always|never` forces or bypasses WAM. [Guide](docs/user-guide.md#windows-wam-broker)
- **Built-in kubelogin** — kubectl authenticates to AKS using this binary alone, with no external kubelogin. [Guide](docs/user-guide.md#built-in-kubelogin)
- **Renaming kubeconfig identifiers** — rename every kubeconfig identifier (including users, and in `-f -` output) so same-named clusters can coexist. [Guide](docs/user-guide.md#renaming-kubeconfig-identifiers)
- **AKS through Azure Bastion (az aks bastion)** — one command opens the Bastion tunnel, writes a kubeconfig and runs `--cmd` or prints an export line. [Guide](docs/user-guide.md#aks-through-azure-bastion-az-aks-bastion)
- **Bastion tunnel tuning** — tunable buffers, 30s WebSocket ping/pong and token-exchange retry for `az network bastion tunnel/ssh` and `az aks bastion`. [Guide](docs/user-guide.md#bastion-tunnel-tuning)
- **Privileged Identity Management (az pim)** — list eligible assignments and activate Azure resource roles and Entra group memberships, which the Python CLI has no command for. [Guide](docs/user-guide.md#privileged-identity-management-az-pim)
- **Support tickets (az support)** — a full-screen TUI to file, read and reply to Azure support tickets for any subscription or tenant, with local service search and pre-filled contact details; flag commands match the Python `support` extension. [Guide](docs/user-guide.md#support-tickets-az-support)
- **Terraform-compatible az version** — `az version` reports a fixed `azure-cli` value for Terraform plus the real `azure-cli-go` version. [Guide](docs/user-guide.md#terraform-compatible-az-version)

## Features

### Authentication & Account Management
- `az login` - Authenticate with Azure using device code flow
- `az logout` - Sign out from Azure
- `az account` - Manage subscriptions and authentication tokens

### Compute
- `az vm` - Manage virtual machines (list, show, start, stop, delete, list-skus)
- `az ssh` - SSH into VMs and Arc servers with Entra ID certificates (vm, config, cert, arc); built in, no extension needed

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
- `az pim` - List and activate eligible PIM role assignments and Entra group memberships

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

### From MSI (Windows)

```powershell
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$assets = (Invoke-RestMethod 'https://api.github.com/repos/dobbo-ca/azure-go-cli/releases/latest').assets
$asset = $assets | Where-Object { $_.name -like 'az-go-*-windows-amd64.msi' } | Select-Object -First 1
if (-not $asset) { throw "No Windows MSI found on the latest release" }
$sha = $assets | Where-Object { $_.name -eq "$($asset.name).sha256" } | Select-Object -First 1
if (-not $sha) { throw "No .sha256 for $($asset.name)" }
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $asset.name
Invoke-WebRequest -Uri $sha.browser_download_url -OutFile $sha.name
if ((Get-FileHash $asset.name -Algorithm SHA256).Hash -ine (Get-Content $sha.name).Trim()) { throw "checksum mismatch" }
$p = Start-Process msiexec.exe -ArgumentList '/i', "`"$PWD\$($asset.name)`"", '/qn', '/norestart' -Wait -Verb RunAs -PassThru
if ($p.ExitCode -ne 0) { throw "msiexec failed with exit code $($p.ExitCode)" }
```

Open a new terminal, then check with `where.exe az` and `az version`. The MSI is not code-signed, so expect a UAC prompt from an unknown publisher.

See [`docs/windows-install.md`](docs/windows-install.md) for silent/logged install flags, upgrade/uninstall, and the Microsoft Azure CLI name-conflict note.

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

#### Isolated sessions with `AZ_SESSION`

```bash
export AZ_SESSION=customer-a
az login   # uses ~/.azure/azureProfile-customer-a.json and its own token cache
```

See [Parallel sign-ins with AZ_SESSION](docs/user-guide.md#parallel-sign-ins-with-az_session) in the user guide.

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

### SSH with Entra ID

```bash
# SSH with a short-lived Entra ID certificate
az ssh vm --ip 10.0.0.4
az ssh vm -g my-rg -n my-vm

# Write a config for plain ssh, MobaXterm, rsync, git
az ssh config --ip 10.0.0.4 --file ./sshconfig
ssh -F ./sshconfig 10.0.0.4
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

### Kubernetes (AKS)

```bash
# Merge cluster credentials into ~/.kube/config
az aks get-credentials --name my-cluster --resource-group my-rg

# Open a tunnel to a private AKS cluster through Azure Bastion
az aks bastion --name my-cluster --resource-group my-rg \
  --bastion /subscriptions/.../bastionHosts/my-bastion
```

#### Renaming kubeconfig identifiers

```bash
az aks get-credentials -n appcluster-prod-usw2-k8s-20251209 -g my-rg \
  --context-regex '^appcluster-(.+)$' --context-replacement 'acme-$1'
```

See [Renaming kubeconfig identifiers](docs/user-guide.md#renaming-kubeconfig-identifiers) in the user guide.

### Privileged Identity Management (PIM)

`az pim` lists and activates eligible PIM assignments — Azure resource roles and Entra ID group memberships — and inherits `AZ_SESSION` isolation so multiple customer sessions stay separated.

```bash
# List eligible and currently-active assignments
az pim list

# Activate a resource role
az pim activate resource --role Contributor --scope "Acme Corp/Acme Production" \
  --ticket Jira:TEC-1234 --justification "Investigating INC-9999" --duration 60
```

Entra group PIM is not supported: Microsoft retired the API it used. See [`docs/pim.md`](docs/pim.md).

See [`docs/pim.md`](docs/pim.md) and the [user guide](docs/user-guide.md#privileged-identity-management-az-pim) for the full guide: the four-form `--scope` resolver, `--set-subscription` behavior, validation pre-flight, AZ_SESSION integration for multi-customer workflows, the vendored upstream attribution, and current limitations.

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
- **Extra capabilities**: See [Capabilities beyond the Python Azure CLI](#capabilities-beyond-the-python-azure-cli)

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
