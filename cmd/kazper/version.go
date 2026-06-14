package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Populated at build time via -ldflags. Defaults make `go run` output
// meaningful even without explicit injection.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build metadata and exit",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "kazper version=%s commit=%s date=%s\n", version, commit, date)
		},
	}
}
