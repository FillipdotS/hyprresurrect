// Package cmd implements the cli commands / flags of hyprresurrect
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hyprresurrect",
	Short: "hyprresurrect restores your hyprland tiles on reboot.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("ran root command")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
