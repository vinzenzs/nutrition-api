// Command kazper is the single binary entrypoint for the project.
// It exposes the HTTP API, the MCP server, schema migrations, and version
// info as Cobra subcommands.
//
// @title           Kazper
// @version         0.1.0
// @description     Kazper — personal endurance-fueling and training-coach REST API. All endpoints under /products, /meals, and /summary require a bearer token.
// @BasePath        /
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Static bearer token. Use the format `Bearer <token>` with either MOBILE_API_TOKEN or AGENT_API_TOKEN.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "kazper",
		Short: "Kazper — endurance-fueling and training-coach API and tooling",
		Long: `kazper bundles the HTTP REST API, the MCP server, and ` +
			`migration tooling behind a single binary. Run with --help on any ` +
			`subcommand for details.`,
		SilenceUsage: true,
		// When no subcommand is supplied, print help and exit non-zero so
		// scripts that accidentally invoke the bare binary fail loudly.
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return fmt.Errorf("a subcommand is required")
		},
	}
	// Reserved for future config-file support; today it's accepted but unused.
	root.PersistentFlags().String("config", "", "path to config file (reserved; not yet used)")

	root.AddCommand(serveCmd())
	root.AddCommand(mcpCmd())
	root.AddCommand(migrateCmd())
	root.AddCommand(versionCmd())
	return root
}
