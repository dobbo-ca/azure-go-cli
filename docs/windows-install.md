# Windows Installation (`az-go` MSI)

A per-machine MSI installer is published on final releases (not betas) for `windows/amd64`. It installs `az.exe` to `C:\Program Files\Azure Go CLI\bin\` and appends that directory to the system `PATH`, supports clean upgrades (just run the newer MSI), and uninstalls cleanly via Add/Remove Programs or `msiexec /x`.

`arm64` users, and anyone who wants a per-user (no admin) install, should use the [`.zip` release](#zip-alternative-no-admin--arm64) instead — see below.

## Quick reference

```powershell
# Download the latest release's MSI, verify it, and install silently.
# -Verb RunAs raises a UAC prompt if this shell isn't already elevated.
$ErrorActionPreference = 'Stop'            # never fall through to the install on an error
$ProgressPreference = 'SilentlyContinue'   # Invoke-WebRequest is slow on 5.1 without this
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

# Refresh PATH in the current session, then verify
$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')
where.exe az
az version
```

## Download

### A specific version

```powershell
$version = 'vX.Y.Z'   # a release tag from https://github.com/dobbo-ca/azure-go-cli/releases that has an .msi asset
$msi = "az-go-$version-windows-amd64.msi"

# Windows PowerShell 5.1's Invoke-WebRequest renders a progress bar per
# chunk, which is very slow over a network. Suppress it first.
$ProgressPreference = 'SilentlyContinue'

Invoke-WebRequest -Uri "https://github.com/dobbo-ca/azure-go-cli/releases/download/$version/$msi" -OutFile $msi
Invoke-WebRequest -Uri "https://github.com/dobbo-ca/azure-go-cli/releases/download/$version/$msi.sha256" -OutFile "$msi.sha256"
```

### The latest release

```powershell
$ProgressPreference = 'SilentlyContinue'

$release = Invoke-RestMethod -Uri 'https://api.github.com/repos/dobbo-ca/azure-go-cli/releases/latest'
$asset = $release.assets | Where-Object { $_.name -like 'az-go-*-windows-amd64.msi' } | Select-Object -First 1
if (-not $asset) { throw "No Windows MSI on the latest release" }
$shaAsset = $release.assets | Where-Object { $_.name -eq "$($asset.name).sha256" } | Select-Object -First 1
if (-not $shaAsset) { throw "No .sha256 for $($asset.name)" }

Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $asset.name
Invoke-WebRequest -Uri $shaAsset.browser_download_url -OutFile $shaAsset.name
```

## Verify the checksum

The `.sha256` asset is a bare hex digest (no filename), so compare it directly against `Get-FileHash`:

```powershell
$msi = 'az-go-vX.Y.Z-windows-amd64.msi'   # the file you downloaded above
$expected = (Get-Content "$msi.sha256").Trim()
$actual = (Get-FileHash $msi -Algorithm SHA256).Hash
if ($actual -ine $expected) { throw "checksum mismatch: expected $expected, got $actual" }
```

`-ine` is a case-insensitive comparison — `Get-FileHash` returns uppercase hex, the `.sha256` file is lowercase.

## Install

### Interactive

Double-click the `.msi`, or:

```powershell
msiexec.exe /i az-go-vX.Y.Z-windows-amd64.msi
```

Windows Installer shows a UAC elevation prompt (see [Unsigned installer](#unsigned-installer) below), then a progress bar. This package has no setup wizard and shows no completion dialog — verify it worked with `where.exe az` in a **new** terminal afterwards.

### Silent, elevated

Run this from an **already-elevated** PowerShell ("Run as administrator"):

```powershell
msiexec.exe /i "C:\path\az-go-vX.Y.Z-windows-amd64.msi" /qn /norestart
```

- `/qn` — no UI, no prompts.
- `/norestart` — never reboot the machine automatically.

### Silent, with a log

```powershell
msiexec.exe /i "C:\path\az-go-vX.Y.Z-windows-amd64.msi" /qn /norestart /l*v "$env:TEMP\az-go-install.log"
```

`/l*v` logs verbosely. The log directory must already exist — `msiexec` will not create one — so `$env:TEMP` is a safe target.

### Self-elevating one-liner

Run from a **normal** (non-elevated) PowerShell. This triggers one UAC prompt and reports the real exit code:

```powershell
$msi = (Resolve-Path .\az-go-vX.Y.Z-windows-amd64.msi).Path
$p = Start-Process msiexec.exe -ArgumentList '/i', "`"$msi`"", '/qn', '/norestart', '/l*v', "`"$env:TEMP\az-go-install.log`"" -Verb RunAs -Wait -PassThru
$p.ExitCode   # 0 = success, 3010 = success but reboot needed, non-zero = failure
```

`-Verb RunAs` raises the UAC prompt. `-Wait -PassThru` are required, not optional: `msiexec.exe` is a GUI-subsystem binary, so calling it directly from PowerShell returns immediately without setting `$LASTEXITCODE`.

## Unsigned installer

**This MSI is not code-signed.** We do not currently hold a Windows code-signing certificate. Expect:

- **UAC prompt with publisher "Unknown".** Every install and uninstall triggers a User Account Control prompt reading that an unidentified program wants to make changes to your computer. This is expected and does not mean the file is corrupt.
- **SmartScreen warning**, if you downloaded the file with a browser: a blue "Windows protected your PC" dialog. Click **More info → Run anyway** to proceed.
- **Smart App Control (Windows 11), if enabled, blocks unsigned executables it has no confident cloud reputation for**, with no per-file override — this applies equally to `az.exe` whether it comes from the MSI or the `.zip`, since SAC evaluates the binary at launch, not the installer. The only fix is to turn Smart App Control off (Windows Security → App & browser control → Smart App Control) before installing; recent Windows builds let you turn it back on afterwards without a clean reinstall.

Downloading with `Invoke-WebRequest`/`Invoke-RestMethod` (as shown above) rather than a browser generally avoids the SmartScreen download warning, because the file isn't tagged with the Mark-of-the-Web the way a browser download is. If you did download it in a browser and hit warnings, `Unblock-File .\az-go-...msi` removes that tag.

Verifying the SHA-256 checksum (above) is the integrity check this release provides in place of a signature — do it before you install.

## After installing: refresh PATH

The installer writes the system `PATH`, but an already-open terminal keeps its own frozen copy of the environment and will not see the change. Either open a new terminal window, or refresh the current session:

```powershell
$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')
```

This rebuilds `PATH` purely from the registry, so it drops anything added to this session only (a venv/conda/nvm activation, `$PROFILE` additions). If that matters, open a new terminal instead.

`cmd.exe` has no equivalent — close and reopen the window.

Then verify:

```powershell
where.exe az
az version
```

`where.exe az` lists every `az` found on `PATH`, in the order it will resolve — the first line is what actually runs.

This CLI has no `--version` flag; use the `version` subcommand. It prints one JSON line, for example:

```json
{"azure-cli": "2.67.0", "azure-cli-go": "v1.7.0", "azure-cli-go-commit": "adb8f9f3c1e04a5d6b2f8790c4de1a6b7f0d2c31", "azure-cli-go-build-date": "2026-08-12T07:55:58Z"}
```

The `azure-cli` field reports a compatible Microsoft version on purpose, because the Terraform `azurerm` provider enforces a minimum. The `azure-cli-go` field is this CLI's own version.

## If you also have the Microsoft Azure CLI installed

Microsoft's `az` (the Python-based Azure CLI) installs `az.cmd` and **prepends** its directory to the system `PATH`. This installer **appends** its directory instead, so on a machine with both installed, `az` resolves to Microsoft's CLI, not this one — regardless of which you installed first or most recently.

To check which one you're getting:

```powershell
where.exe az                 # first line is what actually runs
Get-Command az -All          # shows every match with its source path
az version                   # ours includes an "azure-cli-go" field; Microsoft's does not
```

Both CLIs answer `az version` with JSON, and both report an `azure-cli` field, so that field cannot tell them apart. Only ours prints `azure-cli-go`. Microsoft's also accepts `az --version`; ours rejects it with `unknown flag: --version`, which is itself a quick way to tell which one you have.

Options if you want this CLI to be your `az`:

- Run it by full path: `& "C:\Program Files\Azure Go CLI\bin\az.exe"`.
- Move `C:\Program Files\Azure Go CLI\bin` ahead of Microsoft's `wbin` entry in **System Properties → Environment Variables** → system `Path`. This changes `az` for every user and script on the machine, so do it deliberately.

## Upgrade

Just install the newer MSI — no need to uninstall first. Run this from an **already-elevated** PowerShell, same as [Silent, elevated](#silent-elevated) above:

```powershell
msiexec.exe /i az-go-vX.Y.Z-windows-amd64.msi /qn /norestart
```

The installer removes the old version as part of the same transaction (a "major upgrade"). Downgrading (installing an older version over a newer one) is blocked with an explicit error message.

## Uninstall

### Add/Remove Programs

Settings → Apps → Installed apps → **Azure Go CLI** → Uninstall.

### PowerShell, by ProductCode (recommended)

```powershell
$app = Get-ChildItem 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall',
                     'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall' |
       Get-ItemProperty |
       Where-Object { $_.DisplayName -like 'Azure Go CLI*' } |
       Select-Object -First 1
if (-not $app) { throw "Azure Go CLI is not installed" }
$app | Select-Object DisplayName, DisplayVersion, PSChildName
Start-Process msiexec.exe -ArgumentList '/x', $app.PSChildName, '/qn', '/norestart' -Wait -Verb RunAs
```

### With the original .msi file

Run this from an **already-elevated** PowerShell:

```powershell
msiexec.exe /x az-go-vX.Y.Z-windows-amd64.msi /qn /norestart
```

This only works with the **exact file you installed from** — each build gets a distinct internal ProductCode, so pointing `/x` at a different version's `.msi` fails with error 1605.

Uninstalling removes `az.exe` and the PATH entry. It does **not** touch `%USERPROFILE%\.azure\` (your profile and token cache) — remove that separately if you want a full clean-up.

## Zip alternative (no admin / arm64)

If you can't or don't want to run an MSI — no admin rights, or you're on `arm64` (the MSI is `amd64`-only today) — download the `.zip` instead from the [releases page](https://github.com/dobbo-ca/azure-go-cli/releases), extract `az.exe` anywhere, and add that directory to your own `PATH` (user-level, no admin required). Note this does **not** help with Smart App Control (see [Unsigned installer](#unsigned-installer)) — `az.exe` is unsigned either way.

```powershell
$dir = 'C:\Users\me\tools\az-go'
$cur = [Environment]::GetEnvironmentVariable('Path','User')
[Environment]::SetEnvironmentVariable('Path', $(if ($cur) { "$cur;$dir" } else { $dir }), 'User')
```

## Troubleshooting

**`msiexec` exits with code 1603** — a generic fatal install error. Re-run with `/l*v` logging (above) and check the log's last few `Error` lines. Common causes: an existing install is in a broken state (try uninstalling via ProductCode first), or the process doesn't have the permissions it needs despite the UAC prompt.

**`msiexec` exits with code 1619** — "This installation package could not be opened." The `.msi` file is missing, the path is wrong, or the download was truncated/corrupted. Re-download and re-verify the checksum.

**`msiexec` exits with code 1625** — "This installation is forbidden by system policy." A Group Policy or software restriction policy is blocking Windows Installer itself. Ask your admin. (This is not what Smart App Control blocks — see [Unsigned installer](#unsigned-installer).)

**`az : The term 'az' is not recognized as the name of a cmdlet, function, script file, or operable program`** — the terminal was opened before you installed, or you haven't refreshed `PATH` in this session. Run the [refresh command](#after-installing-refresh-path) above, or open a new terminal window. If that doesn't fix it, run `where.exe az` — if it prints nothing, the install didn't complete or the PATH entry didn't get written; check the install log.

**`az version` prints no `azure-cli-go` field** — you are running Microsoft's Azure CLI, not this one. See [If you also have the Microsoft Azure CLI installed](#if-you-also-have-the-microsoft-azure-cli-installed).

**`Error: unknown flag: --version`** — expected. This CLI has no `--version` flag. Run `az version` instead.

---

See the main [README's Installation section](../README.md#installation) for macOS/Linux install methods.
