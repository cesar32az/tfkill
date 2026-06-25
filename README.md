# tfkill

Interactive TUI for finding and deleting `.terraform` and `.terragrunt-cache` directories. Inspired by [npkill](https://github.com/voidcamels/npkill).

```
  _    __ _    _  _  _
 | |_ / _| | _(_)| || |
 | __| |_| |/ / || || |
 | |_|  _|   <| || || |
  \__|_| |_|\_\_||_||_|

  Releasable:   681.20 MB
  Space saved:    0.0 KB
  Completed in    0.01s

▶ /projects/infra/staging/.terraform ················· 16.40 MB
  /projects/infra/prod/.terraform ··················· 16.40 MB  ⚠  local state
  /projects/platform/k8s/.terraform ················ 648.41 MB

  ↑↓ / jk  navigate  ·  space  delete  ·  q  quit
```

## Installation

Pick the method that matches your environment. Prebuilt binaries are published for
Linux, macOS and Windows on each [release](https://github.com/cesar32az/tfkill/releases).

| Platform | Architectures | Asset |
|----------|---------------|-------|
| Linux | `amd64`, `arm64` | `tfkill_<version>_linux_<arch>.tar.gz` |
| macOS | `amd64` (Intel), `arm64` (Apple Silicon) | `tfkill_<version>_darwin_<arch>.tar.gz` |
| Windows | `amd64` | `tfkill_<version>_windows_amd64.zip` |

### Quick install (Linux / macOS)

Auto-detects the latest version, your OS and architecture, then installs to `/usr/local/bin`:

```bash
VERSION=$(curl -fsSL https://api.github.com/repos/cesar32az/tfkill/releases/latest \
  | grep -oE '"tag_name": *"[^"]+"' | cut -d'"' -f4 | tr -d v)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; esac
curl -fsSL "https://github.com/cesar32az/tfkill/releases/download/v${VERSION}/tfkill_${VERSION}_${OS}_${ARCH}.tar.gz" \
  | tar -xz tfkill
sudo mv tfkill /usr/local/bin/
tfkill --help
```

On macOS the binary is unsigned. If Gatekeeper blocks it, clear the quarantine flag once:

```bash
xattr -d com.apple.quarantine /usr/local/bin/tfkill
```

### Go install

Requires Go 1.21+. Installs into `$(go env GOPATH)/bin` — make sure that directory is on your `PATH`.

```bash
go install github.com/cesar32az/tfkill@latest
```

### Manual download

Grab the asset for your platform from the [releases page](https://github.com/cesar32az/tfkill/releases) and extract it.

**Linux / macOS**
```bash
tar -xzf tfkill_<version>_<os>_<arch>.tar.gz
sudo mv tfkill /usr/local/bin/
```

**Windows (PowerShell)**
```powershell
Expand-Archive tfkill_<version>_windows_amd64.zip -DestinationPath .
# Move tfkill.exe to a directory on your PATH, e.g.:
New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\Programs\tfkill" | Out-Null
Move-Item tfkill.exe "$env:LOCALAPPDATA\Programs\tfkill\"
# Add that directory to your user PATH (one-time):
[Environment]::SetEnvironmentVariable(
  "Path",
  "$([Environment]::GetEnvironmentVariable('Path','User'));$env:LOCALAPPDATA\Programs\tfkill",
  "User")
```

Each release also ships a `tfkill_<version>_checksums.txt` file, so you can verify the
download with `sha256sum -c` (Linux), `shasum -a 256 -c` (macOS) or `Get-FileHash` (Windows).

## Usage

```bash
# Scan current directory
tfkill

# Scan a specific path
tfkill /home/user/projects

# Dry run — report space without deleting anything
tfkill --dry-run
tfkill /home/user/projects --dry-run
```

## Keybindings

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `space` | Delete selected directory |
| `q` / `ctrl+c` | Quit |

When a directory contains a local `terraform.tfstate`, tfkill requires confirmation before deleting (`y` to confirm, `n` / `esc` to cancel).

## How it works

tfkill walks the target directory tree concurrently (worker pool sized to `runtime.NumCPU()`), collects every `.terraform` and `.terragrunt-cache` directory, and presents them in an interactive list. Directories are never deleted without explicit user input. The scan can be cancelled at any time with `q`.

## Building from source

Requires Go 1.21+.

```bash
git clone https://github.com/cesar32az/tfkill.git
cd tfkill
go build -o tfkill .
```

