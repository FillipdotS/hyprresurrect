package cmd

import (
	"fmt"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Saves the current tile layout",
	RunE: func(cmd *cobra.Command, args []string) error {
		return save()
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
}

func save() error {
	s, err := hypr.NewFromEnv()
	if err != nil {
		return err
	}

	snap, err := snapshot.Capture(s)
	if err != nil {
		return err
	}

	fmt.Println(snap.Monitors[0])

	return nil
}
