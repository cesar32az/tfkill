# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is tfkill

A terminal UI tool (inspired by npkill) that recursively scans a directory for `.terraform` and `.terragrunt-cache` folders and lets the user interactively delete them to free disk space. It warns before deleting folders that contain a local `terraform.tfstate`.

```
tfkill [path]          # scan path (defaults to cwd)
tfkill --dry-run       # simulate: mark folders as deleted without touching disk
tfkill /some/path --dry-run
```

## Commands

```bash
# Build
go build -o tfkill .

# Run (scans current working directory)
go run .

# Run tests
go test ./...

# Run a single package's tests
go test ./internal/scanner/...

# Install globally
go install .
```

## Architecture

```
main.go                  → entry point, delegates to cmd.Execute()
cmd/root.go              → Cobra CLI setup; gets cwd, initializes BubbleTea program
internal/scanner/        → filesystem scanning logic
  scanner.go             → Scan() walks dirs concurrently (goroutines + WaitGroup),
                           finds .terraform/.terragrunt-cache, computes size, detects local state
internal/ui/             → BubbleTea TUI
  model.go               → Model/Init/Update/View (Elm architecture)
                           States: isScanning (spinner) → results list → confirmMode (for HasState)
  styles.go              → lipgloss styles (colors, bold, italic)
```

**Data flow:** `cmd` → `ui.InitialModel(dir)` → BubbleTea starts → `Init()` fires `runScan()` as a `tea.Cmd` in the background → `scanFinishedMsg` received in `Update()` → results rendered in `View()`.

**Key interaction states in `model`:**
- `isScanning=true`: shows spinner, blocks all keys except quit
- `isScanning=false, confirmMode=false`: normal list navigation (↑/↓/j/k, space to delete)
- `confirmMode=true`: triggered when selected folder has `HasState=true`; requires y/n confirmation before deletion

## Key dependencies

- `github.com/charmbracelet/bubbletea` — TUI framework (Elm-style Model/Update/View)
- `github.com/charmbracelet/bubbles` — spinner component
- `github.com/charmbracelet/lipgloss` — terminal styling
- `github.com/spf13/cobra` — CLI command structure
