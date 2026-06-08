package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vinzenzs/nutrition-api/internal/config"
	"github.com/vinzenzs/nutrition-api/internal/store"
)

func migrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Apply pending database migrations and exit",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(nil)
			if err != nil {
				return err
			}
			if err := cfg.ValidateForMigrate(); err != nil {
				return err
			}
			if err := store.Migrate(cfg.DatabaseURL); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "migrations applied")
			return nil
		},
	}
}
