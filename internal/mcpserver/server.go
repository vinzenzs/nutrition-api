// Package mcpserver hosts the Model Context Protocol server that exposes
// the nutrition REST API as a set of agent tools. It was previously the
// cmd/mcp package; it now lives behind the `nutrition-api mcp` subcommand.
package mcpserver

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vinzenzs/nutrition-api/internal/config"
)

// Run starts the MCP server over stdio and blocks until ctx is cancelled or
// the server returns. The caller is responsible for installing signal
// handlers that cancel ctx.
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	baseURL, err := cfg.NutritionAPIBaseURL()
	if err != nil {
		return err
	}
	client := newAPIClient(baseURL, cfg.AgentToken, cfg.MCPRequestTimeout)

	// Smoke check the REST API. Log success or failure; do not block tool
	// registration on failure so the agent gets an actionable error on the
	// first tool call rather than the process silently disappearing.
	smokeCtx, smokeCancel := context.WithTimeout(ctx, 2*time.Second)
	if err := client.Healthz(smokeCtx); err != nil {
		logger.Warn("REST API healthz check failed at startup",
			"url", baseURL.String(), "err", err)
	} else {
		logger.Info("REST API healthz ok", "url", baseURL.String())
	}
	smokeCancel()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "nutrition",
		Version: Version,
	}, nil)

	registerProductsTools(server, client)
	registerMealsTools(server, client)
	registerMealPlanTools(server, client)
	registerShoppingTools(server, client)
	registerSummaryTools(server, client)
	registerGoalsTools(server, client)
	registerGoalOverrideTools(server, client)
	registerHydrationTools(server, client)
	registerRacePrepTools(server, client)
	registerRaceTools(server, client)
	registerWorkoutsTools(server, client)
	registerWorkoutTemplateTools(server, client)
	registerTrainingPlanTools(server, client)
	registerWorkoutFuelTools(server, client)
	registerWeightTools(server, client)
	registerRecoveryMetricsTools(server, client)
	registerFitnessMetricsTools(server, client)
	registerHydrationBalanceTools(server, client)
	registerDailySummaryTools(server, client)
	registerGearTools(server, client)
	registerPersonalRecordsTools(server, client)
	registerEnergyTools(server, client)
	registerTrainingPhasesTools(server, client)
	registerDailyContextTools(server, client)
	registerGarminTools(server, client)

	logger.Info("nutrition-mcp ready",
		"version", Version,
		"api_url", baseURL.String(),
	)

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
