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

**Go install**
```bash
go install github.com/cesar32az/tfkill@latest
```

**Download binary** — grab the latest release for your platform from the [releases page](https://github.com/cesar32az/tfkill/releases).

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

