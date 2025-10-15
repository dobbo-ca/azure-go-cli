# azure-go-cli

Azure CLI implemented in Go - lightweight alternative with core authentication commands.

This project provides a minimal implementation of the Azure CLI in Go, focusing on the essential authentication commands. It uses the same Azure device code flow as the official Azure CLI for secure authentication.

## Features

- `az login` - Authenticate with Azure using device code flow
- `az logout` / `az logoff` - Sign out from Azure (removes stored credentials)

## Installation

### From source

```bash
git clone https://github.com/cdobbyn/azure-go-cli.git
cd azure-go-cli
go build -o az
```

### Using go install

```bash
go install github.com/cdobbyn/azure-go-cli@latest
```

## Usage

### Login to Azure

```bash
az login
```

This will display a device code and prompt you to visit https://microsoft.com/devicelogin to complete authentication.

### Logout from Azure

```bash
az logout
# or
az logoff
```

This removes the stored Azure profile from `~/.azure/azureProfile.json`.

## How it works

The CLI uses the Azure SDK for Go (`github.com/Azure/azure-sdk-for-go/sdk/azidentity`) to implement the device code authentication flow. When you run `az login`:

1. A device code is generated and displayed
2. You visit https://microsoft.com/devicelogin and enter the code
3. After successful authentication, a profile is saved to `~/.azure/azureProfile.json`
4. The token information is stored securely by the Azure SDK

When you run `az logout`, the profile file is removed from your system.

## Comparison to Official Azure CLI

This is a minimal implementation that focuses only on authentication. The official Azure CLI (https://github.com/Azure/azure-cli) is a full-featured tool with hundreds of commands for managing Azure resources. Use this project if you need a lightweight Go-based alternative for authentication workflows.

## License

MIT
