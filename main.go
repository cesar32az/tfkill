package main

import "github.com/cesar32az/tfkill/cmd"

// Build metadata, injected at release time by goreleaser via -ldflags -X.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}
