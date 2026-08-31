package cmd

import (
	"errors"
	"fmt"

	"github.com/FillipdotS/hyprresurrect/internal/restore"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
	"github.com/spf13/cobra"
)

var dryRun bool

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restores the most recent saved tile layout",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRestore()
	},
}

func init() {
	restoreCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the hyprland commands instead of running them")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore() error {
	store, err := snapshot.New()
	if err != nil {
		return err
	}

	entries, err := store.List()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return errors.New("no snapshots saved yet; run `hyprresurrect save` first")
	}

	newest := entries[0]

	snap, err := store.Load(newest.Path)
	if err != nil {
		return err
	}

	steps := restore.Plan(snap)

	if !dryRun {
		// TODO
		return errors.New("restore is not implemented yet; re-run with --dry-run")
	}

	fmt.Printf("%s: %d window(s) captured %s\n",
		newest.Path, newest.Windows, newest.CapturedAt.Format("2006-01-02 15:04:05"),
	)

	if len(steps) == 0 {
		fmt.Println("nothing to restore")
		return nil
	}

	for _, step := range steps {
		fmt.Printf("\n-- %s\n%s\n", step.What, step.Lua)
	}

	return nil
}
