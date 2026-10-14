package cmd

import (
	"fmt"
	"os"

	"github.com/cesar32az/tfkill/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var dryRun bool

var rootCmd = &cobra.Command{
	Use:   "tfkill [path]",
	Short: "Find and delete .terraform folders interactively",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var dir string
		if len(args) == 1 {
			dir = args[0]
		} else {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
		}

		m := ui.InitialModel(dir, dryRun)
		// Alt screen gives the TUI a fixed-size canvas so long result lists
		// scroll inside the window instead of pushing the header off-screen.
		p := tea.NewProgram(m, tea.WithAltScreen())
		final, err := p.Run()
		if err != nil {
			fmt.Printf("Error starting the UI: %v", err)
			os.Exit(1)
		}
		// The alt screen is cleared on exit; print a short summary instead.
		if s, ok := final.(interface{ Summary() string }); ok {
			fmt.Println(s.Summary())
		}
	},
}

func init() {
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Scan and report space without deleting anything")
}

// SetVersionInfo wires the build metadata into the root command so that
// `tfkill --version` reports the version, commit and build date.
func SetVersionInfo(version, commit, date string) {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(
		fmt.Sprintf("tfkill %s\ncommit: %s\nbuilt:  %s\n", version, commit, date))
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
