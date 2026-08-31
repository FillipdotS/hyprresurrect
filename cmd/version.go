package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Overridable at build time with:
// -ldflags "-X github.com/FillipdotS/hyprresurrect/cmd.version=v1.2.3"
var version string

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the current version of hyprresurrect",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(buildVersion())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func buildVersion() string {
	if version != "" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "devel"
	}
	return info.Main.Version
}
