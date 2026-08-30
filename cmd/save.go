package cmd

import (
	"fmt"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
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

	if err := s.Notify("hyprresurrect - saving..."); err != nil {
		return err
	}

	clients, err := s.Clients()
	if err != nil {
		return err
	}

	fmt.Println(clients)

	return nil
}
