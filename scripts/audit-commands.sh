#!/bin/bash
# Script to audit Azure CLI commands and compare with our implementation

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="${SCRIPT_DIR}/../docs"
mkdir -p "${OUTPUT_DIR}"

OFFICIAL_COMMANDS="${OUTPUT_DIR}/official-az-commands.txt"
IMPLEMENTED_COMMANDS="${OUTPUT_DIR}/implemented-commands.txt"
MISSING_COMMANDS="${OUTPUT_DIR}/missing-commands.txt"
COMMAND_TREE="${OUTPUT_DIR}/command-tree.md"

echo "Discovering official Azure CLI commands..."

# Check if we can use docker or podman
CONTAINER_CMD=""
if command -v docker &> /dev/null; then
  CONTAINER_CMD="docker"
elif command -v podman &> /dev/null; then
  CONTAINER_CMD="podman"
else
  echo "Error: Neither docker nor podman found. Please install one to run this audit."
  exit 1
fi

echo "Using container runtime: ${CONTAINER_CMD}"

# Pull the official Azure CLI image if not present
echo "Ensuring Azure CLI container image is available..."
${CONTAINER_CMD} pull mcr.microsoft.com/azure-cli:latest >/dev/null 2>&1

# Get top-level command groups
echo "Extracting command tree (this may take a few minutes)..."
> "${OFFICIAL_COMMANDS}"

# Get main commands
${CONTAINER_CMD} run --rm mcr.microsoft.com/azure-cli:latest az --help 2>/dev/null | \
  awk '/^(Subgroups|Commands):$/,/^$/ {
    # Match lines that start with spaces followed by a command name and colon
    if ($1 ~ /^[a-z][a-z0-9-]*$/ && $2 == ":") {
      print "az " $1
    }
  }' >> "${OFFICIAL_COMMANDS}"

# For each main command, get its subcommands (limit depth to 2 levels)
for main_cmd in $(cat "${OFFICIAL_COMMANDS}" | cut -d' ' -f2 | sort -u); do
  echo "  Processing az ${main_cmd}..."

  # Get level 1 subcommands
  ${CONTAINER_CMD} run --rm mcr.microsoft.com/azure-cli:latest az ${main_cmd} --help 2>/dev/null | \
    awk -v cmd="az ${main_cmd}" '/^(Subgroups|Commands):$/,/^$/ {
      if ($1 ~ /^[a-z][a-z0-9-]*$/ && $2 == ":") {
        print cmd " " $1
      }
    }' >> "${OFFICIAL_COMMANDS}" || true

  # Get level 2 subcommands for each level 1 command
  for sub_cmd in $(grep "^az ${main_cmd} " "${OFFICIAL_COMMANDS}" | cut -d' ' -f3 | sort -u); do
    ${CONTAINER_CMD} run --rm mcr.microsoft.com/azure-cli:latest az ${main_cmd} ${sub_cmd} --help 2>/dev/null | \
      awk -v cmd="az ${main_cmd} ${sub_cmd}" '/^(Subgroups|Commands):$/,/^$/ {
        if ($1 ~ /^[a-z][a-z0-9-]*$/ && $2 == ":") {
          print cmd " " $1
        }
      }' >> "${OFFICIAL_COMMANDS}" || true
  done
done

# Sort and deduplicate
sort -u "${OFFICIAL_COMMANDS}" -o "${OFFICIAL_COMMANDS}"

echo "Found $(wc -l < "${OFFICIAL_COMMANDS}") official commands"

# Get our implemented commands from the binary
echo "Extracting implemented commands..."
if [[ ! -f "${SCRIPT_DIR}/../bin/az/az" ]]; then
  echo "Building az binary first..."
  make -C "${SCRIPT_DIR}/.." build >/dev/null
fi

> "${IMPLEMENTED_COMMANDS}"

# Get main commands
"${SCRIPT_DIR}/../bin/az/az" --help 2>/dev/null | \
  awk '/^Available Commands:$/,/^Flags:$/ {if ($1 ~ /^[a-z]/ && $1 !~ /:$/) print "az " $1}' >> "${IMPLEMENTED_COMMANDS}"

# For each main command, get its subcommands
for main_cmd in $(cat "${IMPLEMENTED_COMMANDS}" | cut -d' ' -f2 | sort -u); do
  # Get level 1 subcommands
  "${SCRIPT_DIR}/../bin/az/az" ${main_cmd} --help 2>/dev/null | \
    awk -v cmd="az ${main_cmd}" '/^Available Commands:$/,/^Flags:$/ {if ($1 ~ /^[a-z]/ && $1 !~ /:$/) print cmd " " $1}' >> "${IMPLEMENTED_COMMANDS}" || true

  # Get level 2 subcommands for each level 1 command
  for sub_cmd in $(grep "^az ${main_cmd} " "${IMPLEMENTED_COMMANDS}" 2>/dev/null | cut -d' ' -f3 | sort -u); do
    "${SCRIPT_DIR}/../bin/az/az" ${main_cmd} ${sub_cmd} --help 2>/dev/null | \
      awk -v cmd="az ${main_cmd} ${sub_cmd}" '/^Available Commands:$/,/^Flags:$/ {if ($1 ~ /^[a-z]/ && $1 !~ /:$/) print cmd " " $1}' >> "${IMPLEMENTED_COMMANDS}" || true
  done
done

# Sort and deduplicate
sort -u "${IMPLEMENTED_COMMANDS}" -o "${IMPLEMENTED_COMMANDS}"

echo "Found $(wc -l < "${IMPLEMENTED_COMMANDS}") implemented commands"

# Compare and find missing commands
echo "Comparing command sets..."
comm -23 "${OFFICIAL_COMMANDS}" "${IMPLEMENTED_COMMANDS}" > "${MISSING_COMMANDS}"

echo "Found $(wc -l < "${MISSING_COMMANDS}") missing commands"

# Generate a markdown tree structure
echo "Generating command tree..."

cat > "${COMMAND_TREE}" << 'EOF'
# Azure CLI Command Coverage

This document shows the command tree for Azure CLI and tracks implementation status.

Legend:
- ✅ Implemented
- ❌ Not implemented

EOF

# Add summary
total=$(wc -l < "${OFFICIAL_COMMANDS}")
implemented=$(wc -l < "${IMPLEMENTED_COMMANDS}")
missing=$(wc -l < "${MISSING_COMMANDS}")
percentage=$(awk "BEGIN {printf \"%.1f\", ($implemented/$total)*100}")

cat >> "${COMMAND_TREE}" << EOF
## Summary

- **Total Commands**: ${total}
- **Implemented**: ${implemented} (${percentage}%)
- **Missing**: ${missing}

---

## Command Tree

EOF

# Build tree output
current_group=""
while IFS= read -r cmd; do
  # Check if implemented
  if grep -q "^${cmd}$" "${IMPLEMENTED_COMMANDS}"; then
    status="✅"
  else
    status="❌"
  fi

  # Extract the top-level command group
  group=$(echo "$cmd" | cut -d' ' -f1-2)

  if [[ "$group" != "$current_group" ]]; then
    current_group="$group"
    echo "" >> "${COMMAND_TREE}"
    echo "### ${group}" >> "${COMMAND_TREE}"
    echo "" >> "${COMMAND_TREE}"
  fi

  # Calculate indentation based on depth
  depth=$(echo "$cmd" | awk '{print NF-1}')
  indent=""
  for ((i=0; i<depth-1; i++)); do
    indent="${indent}  "
  done

  # Get just the command name (last part)
  cmd_name=$(echo "$cmd" | awk '{print $NF}')

  echo "${indent}- ${status} \`${cmd_name}\`" >> "${COMMAND_TREE}"
done < "${OFFICIAL_COMMANDS}"

echo ""
echo "✅ Audit complete!"
echo ""
echo "Generated files:"
echo "  - ${OFFICIAL_COMMANDS}"
echo "  - ${IMPLEMENTED_COMMANDS}"
echo "  - ${MISSING_COMMANDS}"
echo "  - ${COMMAND_TREE}"
echo ""
echo "Summary: ${implemented}/${total} commands implemented (${percentage}%)"
