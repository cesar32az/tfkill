package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/cesar32az/tfkill/internal/ui"
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
		p := tea.NewProgram(m)
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error starting the UI: %v", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Scan and report space without deleting anything")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

