# Azure Go CLI - Development Guide

## Building the Project

**ALWAYS use `make build` instead of direct `go build` commands.**

The binary should be output to `bin/az/az`, not to the project root.

### Build Commands

```bash
# Build for current OS/architecture (outputs to bin/az/az)
make build

# Build for all supported platforms
make all

# Run tests
make test

# Clean build artifacts
make clean

# Install to system (/usr/local/bin)
make install
```

## Project Structure

```
azure-go-cli/
├── cmd/az/              # Main entry point
├── internal/            # Internal packages
│   ├── account/        # Account management commands
│   ├── aks/            # AKS commands
│   ├── auth/           # Authentication (login/logout)
│   ├── group/          # Resource group commands
│   ├── keyvault/       # Key Vault commands
│   ├── network/        # Network commands
│   ├── postgres/       # PostgreSQL commands
│   ├── storage/        # Storage commands
│   └── vm/             # Virtual machine commands
├── pkg/                # Public packages
│   ├── azure/          # Azure credential helpers
│   └── config/         # Configuration management
└── bin/                # Build output directory (gitignored)
```

## Adding New Commands

When adding a new command domain (e.g., `vm`):

1. Create package directory: `internal/{domain}/`
2. Create `commands.go` with cobra command structure
3. Implement individual command functions (e.g., `list.go`, `show.go`)
4. Import and register in `cmd/az/main.go`
5. Run `go mod tidy` to update dependencies
6. Build with `make build`

## Code Style

- Use 2 spaces for indentation
- All files must end with newline (LF)
- Follow Go standard naming conventions
- Use context.Context for all Azure SDK calls
- Output JSON by default, table format as option

## Testing

```bash
# Run all tests
make test

# Test specific command manually
./bin/az/az {command} --help
./bin/az/az {command} {subcommand} {flags}
```

## Dependencies

The project uses:
- `github.com/Azure/azure-sdk-for-go/sdk/*` - Azure SDK for Go
- `github.com/spf13/cobra` - CLI framework
- Standard library for most operations
