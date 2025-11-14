# Azure Go CLI Scripts

This directory contains helper scripts for developing and auditing the Azure Go CLI.

## Scripts

### audit-commands.sh

Discovers all Azure CLI commands from the official CLI and compares them with our implementation to generate a comprehensive backlog.

**Usage:**

```bash
./scripts/audit-commands.sh
```

**Output:**

The script generates four files in the `docs/` directory:

1. `official-az-commands.txt` - Complete list of official Azure CLI commands
2. `implemented-commands.txt` - Commands we've implemented
3. `missing-commands.txt` - Commands we haven't implemented yet
4. `command-tree.md` - Markdown document with visual tree and status indicators

**Requirements:**

- Docker or Podman installed
- Internet connection (to pull Azure CLI container image)

### az-official

Wrapper script to run the official Azure CLI via container. Useful for:

- Checking `--help` output when implementing new commands
- Running commands not yet implemented in azure-go-cli
- Verifying behavior of official CLI

**Usage:**

```bash
# Check help for a command you're implementing
./scripts/az-official network vnet --help

# Run a command not yet implemented
./scripts/az-official policy assignment list

# Works with all official az commands
./scripts/az-official <command> [args...]
```

**Features:**

- Automatically mounts your `~/.azure` config directory for authentication
- Uses Docker or Podman (whichever is available)
- Provides same environment as official CLI

## Workflow for Adding New Commands

1. **Audit current state:**
   ```bash
   ./scripts/audit-commands.sh
   ```

2. **Check the backlog:**
   ```bash
   cat docs/command-tree.md
   ```

3. **Research command details:**
   ```bash
   ./scripts/az-official <command> --help
   ```

4. **Implement the command** in azure-go-cli

5. **Re-audit to update status:**
   ```bash
   ./scripts/audit-commands.sh
   ```

## Replacing Official CLI

Once you're confident in the implementation coverage, you can replace the official CLI:

```bash
# Backup the official CLI (if installed via package manager)
sudo mv /usr/local/bin/az /usr/local/bin/az.official

# Install azure-go-cli
make install

# Use scripts/az-official for any unimplemented commands
alias az-official="$PWD/scripts/az-official"
```

## Notes

- The audit script may take 5-10 minutes to complete as it queries the official CLI for all command help output
- The official Azure CLI container image is ~1.2GB
- Results are cached in `docs/` directory and can be committed to track progress over time
