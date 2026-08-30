package cmd

import (
	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Saves the current tile layout",
	Run: func(cmd *cobra.Command, args []string) {
		save()
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
}

func save() {
	hypr.Notify("hyprresurrect - saving layout...")
}
