package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/cesar32az/tfkill/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:   "tfkill",
	Short: "Find and delete .terraform folders interactively",
	Run: func(cmd *cobra.Command, args []string) {
		dir, err := os.Getwd()
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		// Start the UI passing only the start directory
		m := ui.InitialModel(dir)
		p := tea.NewProgram(m)
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error starting the UI: %v", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

