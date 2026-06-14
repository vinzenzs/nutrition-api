package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	_ "github.com/vinzenzs/kazper/docs"
	"github.com/vinzenzs/kazper/internal/config"
	"github.com/vinzenzs/kazper/internal/httpserver"
)

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP REST API",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := config.New()
			if err := config.BindFlags(v, cmd.Flags()); err != nil {
				return err
			}
			cfg, err := config.Load(v)
			if err != nil {
				return err
			}
			if err := cfg.ValidateForServe(); err != nil {
				return err
			}

			logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			slog.SetDefault(logger)

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			return httpserver.Run(ctx, cfg, logger)
		},
	}
	cmd.Flags().String("addr", "", "HTTP listen address (overrides HTTP_ADDR)")
	return cmd
}
