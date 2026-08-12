#!/usr/bin/env bash
# build-msi.sh - build a per-machine Windows MSI installer for Azure Go CLI
# using GNOME msitools `wixl` against packaging/windows/az-go.wxs.
#
# Usage:
#   build-msi.sh --version <v1.7.0|1.7.0> --exe <path to az.exe> --arch <amd64> --out <output dir>
#
# Flags:
#   --version   Release tag or bare semver, e.g. v1.7.0 or 1.7.0. A leading "v"
#               is stripped to derive the MSI ProductVersion, but the file name
#               keeps the string as given, so the .msi matches the sibling
#               .zip/.tar.gz assets of the same release. Prerelease versions
#               (containing "-", e.g. v1.7.0-beta.99) are REJECTED: MSI
#               ProductVersion cannot express prerelease semantics.
#   --exe       Path to the built windows/<arch> az.exe to embed in the MSI.
#   --arch      Target architecture. Only "amd64" is currently supported
#               (wixl 0.106 does not support arm64; see case block below).
#   --out       Output directory for the .msi and .msi.sha256 files.
#   -h, --help  Show this help and exit.
#
# Output:
#   <out>/az-go-<tag>-windows-<arch>.msi
#   <out>/az-go-<tag>-windows-<arch>.msi.sha256   (bare hex digest, no filename)
set -euo pipefail

usage() {
  # Print the header comment block (everything between the shebang and the
  # first line of code), so the help text cannot drift out of sync with it.
  awk 'NR == 1 { next } /^#/ { sub(/^# ?/, ""); print; next } { exit }' "$0"
}

VERSION=""
EXE=""
ARCH=""
OUT=""

while [ $# -gt 0 ]; do
  case "$1" in
    --version) [ $# -ge 2 ] || { echo "error: --version requires a value" >&2; exit 1; }; VERSION="$2"; shift 2 ;;
    --exe) [ $# -ge 2 ] || { echo "error: --exe requires a value" >&2; exit 1; }; EXE="$2"; shift 2 ;;
    --arch) [ $# -ge 2 ] || { echo "error: --arch requires a value" >&2; exit 1; }; ARCH="$2"; shift 2 ;;
    --out) [ $# -ge 2 ] || { echo "error: --out requires a value" >&2; exit 1; }; OUT="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "error: unknown argument: $1" >&2; usage >&2; exit 1 ;;
  esac
done

if [ -z "$VERSION" ] || [ -z "$EXE" ] || [ -z "$ARCH" ] || [ -z "$OUT" ]; then
  echo "error: --version, --exe, --arch and --out are all required" >&2
  usage >&2
  exit 1
fi

if ! command -v wixl >/dev/null 2>&1; then
  echo "error: wixl not found on PATH. Install msitools (macOS: brew install msitools; Debian/Ubuntu >= 25.10: apt-get install wixl msitools)." >&2
  exit 1
fi

if [ ! -f "$EXE" ]; then
  echo "error: --exe path does not exist: $EXE" >&2
  exit 1
fi

# PV is the numeric MSI ProductVersion; TAG is kept verbatim so the .msi file
# name matches the other assets of the same release, whichever spelling the
# release tag uses.
PV="${VERSION#v}"
TAG="$VERSION"

case "$PV" in
  *-*)
    echo "error: refusing to build an MSI for prerelease version '$PV' (MSI ProductVersion cannot express prerelease semantics)" >&2
    exit 1
    ;;
esac

IFS=. read -r MA MI BU EXTRA <<EOF
$PV
EOF

if [ -z "$MA" ] || [ -z "$MI" ] || [ -z "$BU" ]; then
  echo "error: --version '$VERSION' is not in major.minor.build form" >&2
  exit 1
fi

if [ -n "$EXTRA" ]; then
  echo "error: --version '$VERSION' has more than three components; Windows Installer ignores anything past major.minor.build" >&2
  exit 1
fi

case "$MA" in ''|*[!0-9]*) echo "error: version major component '$MA' is not numeric" >&2; exit 1 ;; esac
case "$MI" in ''|*[!0-9]*) echo "error: version minor component '$MI' is not numeric" >&2; exit 1 ;; esac
case "$BU" in ''|*[!0-9]*) echo "error: version build component '$BU' is not numeric" >&2; exit 1 ;; esac

if [ "$MA" -gt 255 ] || [ "$MI" -gt 255 ] || [ "$BU" -gt 65535 ]; then
  echo "error: version '$PV' is out of MSI ProductVersion range (max 255.255.65535)" >&2
  exit 1
fi

case "$ARCH" in
  amd64)
    WIXL_ARCH=x64
    PFOLDER=ProgramFiles64Folder
    ;;
  *)
    echo "error: unsupported arch '$ARCH' (only amd64 is supported; wixl 0.106 has no arm64 support yet)" >&2
    exit 1
    ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WXS="$SCRIPT_DIR/az-go.wxs"

if [ ! -f "$WXS" ]; then
  echo "error: wxs source not found: $WXS" >&2
  exit 1
fi

mkdir -p "$OUT"

MSI_NAME="az-go-${TAG}-windows-${ARCH}.msi"
MSI_PATH="$OUT/$MSI_NAME"

echo "Building $MSI_NAME (ProductVersion $PV, arch $WIXL_ARCH)..."
wixl -a "$WIXL_ARCH" \
     -D Version="$PV" \
     -D BinPath="$EXE" \
     -D PFolder="$PFOLDER" \
     -o "$MSI_PATH" \
     "$WXS"

if command -v shasum >/dev/null 2>&1; then
  shasum -a 256 "$MSI_PATH" | awk '{print $1}' > "$MSI_PATH.sha256"
elif command -v sha256sum >/dev/null 2>&1; then
  sha256sum "$MSI_PATH" | awk '{print $1}' > "$MSI_PATH.sha256"
else
  echo "error: neither shasum nor sha256sum is available" >&2
  exit 1
fi

echo "Wrote $MSI_PATH"
echo "Wrote $MSI_PATH.sha256"
