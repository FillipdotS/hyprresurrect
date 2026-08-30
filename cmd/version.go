package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the current version of hyprresurrect",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("TODO - add version num")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
