# Contributing to tfkill

Thanks for your interest in improving tfkill. This guide covers how to build,
run, test, and release the project.

## Prerequisites

- [Go](https://go.dev/dl/) 1.21 or newer (the module currently targets the
  version pinned in [`go.mod`](go.mod)).
- `git`.

## Getting started

```bash
git clone https://github.com/cesar32az/tfkill.git
cd tfkill
go mod download
```

## Build

```bash
# Build a local binary
go build -o tfkill .

# Build with version metadata (matches what releases embed)
go build -ldflags "-X main.version=dev -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o tfkill .

./tfkill --version
```

## Run

```bash
# Scan the current directory
go run .

# Scan a specific path
go run . /path/to/projects

# Dry run — report space without deleting anything
go run . --dry-run
```

## Tests

```bash
# Run the whole suite
go test ./...

# Run a single package
go test ./internal/scanner/...

# With race detector and verbose output
go test -race -v ./...
```

There is no test suite yet — contributions that add coverage (especially around
the scanner's path handling and deletion logic) are very welcome. New behavior
should ship with tests.

Before opening a pull request, make sure the basics pass:

```bash
go vet ./...
go build ./...
go test ./...
```

## Commit messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/).
The commit history drives automated versioning, so the prefix matters:

| Prefix | Example | Release effect |
|--------|---------|----------------|
| `fix:` | `fix(scanner): follow symlink only inside root` | patch (`1.2.3` → `1.2.4`) |
| `feat:` | `feat: add --version flag` | minor (`1.2.3` → `1.3.0`) |
| `feat!:` / `BREAKING CHANGE:` footer | `feat!: drop windows arm64` | major (`1.2.3` → `2.0.0`) |
| `docs:`, `chore:`, `refactor:`, `test:`, `ci:` | `docs: expand install instructions` | no release |

Keep the subject line in the imperative mood and under ~50 characters; add a
body when the *why* isn't obvious from the subject.

## Pull requests

1. Branch from `main` using a descriptive name (`feat/...`, `fix/...`, `docs/...`).
2. Make your change, keeping commits focused and conventional.
3. Run `go vet`, `go build`, and `go test`.
4. Open the PR against `main` and describe what changed and why.

## Releases

Releases are fully automated — **maintainers do not tag or build manually.**

When commits land on `main`, the [`Release` workflow](.github/workflows/release.yml)
runs [semantic-release](https://semantic-release.gitbook.io/):

1. **Analyze commits** since the last release to compute the next version from
   the Conventional Commit prefixes above.
2. **Update [`CHANGELOG.md`](CHANGELOG.md)** and commit it back with a
   `chore(release): <version> [skip ci]` message.
3. **Tag** the release (`vX.Y.Z`).
4. **Build and publish** cross-platform binaries via
   [GoReleaser](https://goreleaser.com/) (see [`.goreleaser.yml`](.goreleaser.yml)) —
   Linux, macOS and Windows archives plus a checksums file — and attach them to
   the GitHub release.

If a merge to `main` contains only non-releasing commits (`docs:`, `chore:`,
etc.), no release is produced. To cut a release, make sure a `fix:` or `feat:`
commit is included.
