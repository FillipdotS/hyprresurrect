package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/restore"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
	"github.com/spf13/cobra"
)

var (
	dryRun bool
	settle time.Duration
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restores the most recent saved tile layout",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRestore()
	},
}

func init() {
	restoreCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the hyprland commands instead of running them")
	restoreCmd.Flags().DurationVar(&settle, "settle", 2*time.Second, "how long to let windows appear before fixing their placement")
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

	fmt.Printf("%s: %d window(s) captured %s\n",
		newest.Path, newest.Windows, newest.CapturedAt.Format("2006-01-02 15:04:05"),
	)

	runner := restore.Runner{
		Settle: settle,
		Out:    os.Stdout,
		DryRun: dryRun,
	}

	// A dry run never talks to hyprland, so it works outside a session too.
	if !dryRun {
		runner.Hypr, err = hypr.NewFromEnv()
		if err != nil {
			return err
		}
	}

	return runner.Run(snap)
}
