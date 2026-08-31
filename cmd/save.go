package cmd

import (
	"fmt"
	"os"

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

	store, err := snapshot.New()
	if err != nil {
		return err
	}

	path, err := store.Save(snap)
	if err != nil {
		return err
	}

	if err := store.Prune(snapshot.DefaultKeep); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not prune old snapshots: %v\n", err)
	}

	fmt.Printf("saved %d window(s), %d workspace(s), %d monitor(s) to %s\n",
		len(snap.Windows), workspaces(snap), len(snap.Monitors), path,
	)

	return nil
}

func workspaces(snap snapshot.Snapshot) int {
	seen := make(map[int]struct{}, len(snap.Windows))
	for _, w := range snap.Windows {
		seen[w.Workspace] = struct{}{}
	}

	return len(seen)
}
