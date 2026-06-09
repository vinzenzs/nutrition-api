// Package httpserver wires the Gin HTTP API. It was previously the cmd/api
// main package; it now lives behind the `nutrition-api serve` subcommand.
package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/nutrition-api/internal/auth"
	"github.com/vinzenzs/nutrition-api/internal/bodyweight"
	"github.com/vinzenzs/nutrition-api/internal/config"
	"github.com/vinzenzs/nutrition-api/internal/dailycontext"
	"github.com/vinzenzs/nutrition-api/internal/energy"
	"github.com/vinzenzs/nutrition-api/internal/goals"
	"github.com/vinzenzs/nutrition-api/internal/hydration"
	"github.com/vinzenzs/nutrition-api/internal/idempotency"
	"github.com/vinzenzs/nutrition-api/internal/meals"
	"github.com/vinzenzs/nutrition-api/internal/off"
	"github.com/vinzenzs/nutrition-api/internal/products"
	"github.com/vinzenzs/nutrition-api/internal/raceprep"
	"github.com/vinzenzs/nutrition-api/internal/store"
	"github.com/vinzenzs/nutrition-api/internal/summary"
	"github.com/vinzenzs/nutrition-api/internal/trainingphases"
	"github.com/vinzenzs/nutrition-api/internal/workoutfuel"
	"github.com/vinzenzs/nutrition-api/internal/workoutfueling"
	"github.com/vinzenzs/nutrition-api/internal/workouts"
)

// BuildEngine returns a fresh *gin.Engine wired with the framework-level
// defaults the production server uses: Recovery, the JSON NoRoute/NoMethod
// responders documented by the http-error-shape capability, and the
// HandleMethodNotAllowed flag that makes NoMethod fire. Routes are NOT
// registered here — callers add their own. Exposed so tests can drive the
// framework-level invariants without booting the whole server.
func BuildEngine() *gin.Engine {
	r := gin.New()
	// HandleMethodNotAllowed must be set before any route registration so the
	// router builds the per-path method table; without it NoMethod never fires
	// and wrong-method requests fall through to NoRoute as 404 instead of 405.
	r.HandleMethodNotAllowed = true
	r.Use(gin.Recovery())
	// JSON-everywhere error invariant: unknown paths and wrong methods get a
	// structured body instead of Gin's plain-text default. See the
	// http-error-shape capability.
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	})
	r.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method_not_allowed"})
	})
	return r
}

// Run boots the HTTP API and blocks until ctx is cancelled. The caller is
// responsible for installing signal handlers that cancel ctx.
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	authCfg := auth.Config{MobileToken: cfg.MobileToken, AgentToken: cfg.AgentToken}
	if err := authCfg.Validate(); err != nil {
		return err
	}

	if cfg.MigrateOnStart {
		logger.Info("running migrations")
		if err := store.Migrate(cfg.DatabaseURL); err != nil {
			return err
		}
	}

	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	offClient, err := off.New(off.Config{
		Timeout: cfg.OFFTimeout,
		Contact: cfg.OFFUserAgentContact,
	}, logger)
	if err != nil {
		return err
	}

	productsRepo := products.NewRepo(pool)
	productsSvc := products.NewService(pool, productsRepo, offClient)
	mealsRepo := meals.NewRepo(pool)
	mealsSvc := meals.NewService(pool, mealsRepo, productsRepo)
	goalsRepo := goals.NewRepo(pool)
	goalsOverridesRepo := goals.NewOverridesRepo(pool)
	templatesRepo := trainingphases.NewTemplatesRepo(pool)
	templatesSvc := trainingphases.NewTemplatesService(templatesRepo)
	phasesRepo := trainingphases.NewPhasesRepo(pool)
	phasesSvc := trainingphases.NewPhasesService(phasesRepo, templatesRepo)
	goalsResolver := goals.NewResolver(
		goalsRepo, goalsOverridesRepo,
		trainingphases.NewPhaseLookupAdapter(phasesRepo),
		trainingphases.NewTemplateLookupAdapter(templatesRepo),
	)
	summarySvc := summary.NewService(pool, mealsRepo, goalsResolver)
	hydrationRepo := hydration.NewRepo(pool)
	hydrationSvc := hydration.NewService(hydrationRepo)
	workoutsRepo := workouts.NewRepo(pool)
	workoutsSvc := workouts.NewService(workoutsRepo)
	// Wire workouts existence-checks into meals + hydration services so the
	// optional workout_id link is validated before insert/patch (added by
	// add-meal-workout-link).
	mealsSvc.SetWorkoutsRepo(workoutsRepo)
	hydrationSvc.SetWorkoutsRepo(workoutsRepo)
	workoutFuelRepo := workoutfuel.NewRepo(pool)
	workoutFuelSvc := workoutfuel.NewService(workoutFuelRepo)
	workoutFuelSvc.SetWorkoutsRepo(workoutsRepo)
	fuelingSvc := workoutfueling.NewService(workoutsRepo, mealsRepo, hydrationRepo, workoutFuelRepo)
	bodyWeightRepo := bodyweight.NewRepo(pool)
	bodyWeightSvc := bodyweight.NewService(bodyWeightRepo)
	energySvc := energy.NewService(mealsRepo, workoutsRepo, bodyWeightRepo)
	// Protein-distribution needs to resolve weight at the queried date. Same
	// optional-setter pattern that meals/hydration use for SetWorkoutsRepo
	// (add-meal-workout-link).
	summarySvc.SetBodyWeightRepo(bodyWeightRepo)
	userTZ, err := time.LoadLocation(cfg.DefaultUserTZ)
	if err != nil {
		return err
	}
	racePrepSvc := raceprep.NewService(time.Now, userTZ, pool)
	idempRepo := idempotency.NewRepo(pool)

	cleanupCtx, cleanupCancel := context.WithCancel(ctx)
	defer cleanupCancel()
	go idempotency.RunCleanup(cleanupCtx, idempRepo, cfg.IdempotencyTTL, 15*time.Minute, logger)

	gin.SetMode(gin.ReleaseMode)
	r := BuildEngine()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		rctx, rcancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer rcancel()
		if err := pool.Ping(rctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	RegisterSwagger(r, cfg.SwaggerEnabled)

	api := r.Group("/")
	api.Use(requestLogger(logger))
	api.Use(auth.Middleware(authCfg))
	api.Use(idempotency.Middleware(idempRepo, cfg.IdempotencyTTL))

	products.NewHandlers(productsSvc).Register(api)
	meals.NewHandlers(mealsSvc).Register(api)
	summary.NewHandlers(summarySvc, cfg.DefaultUserTZ, logger).Register(api)
	goals.NewHandlers(goalsRepo).Register(api)
	goals.NewOverridesHandlers(goalsOverridesRepo).Register(api)
	trainingphases.NewTemplatesHandlers(templatesSvc).Register(api)
	trainingphases.NewPhasesHandlers(phasesSvc).Register(api)
	hydration.NewHandlers(hydrationSvc).Register(api)
	hydration.NewSummaryHandlers(hydrationSvc, cfg.DefaultUserTZ, logger).Register(api)
	racePrepHandlers := raceprep.NewHandlers(racePrepSvc)
	racePrepHandlers.SetLogger(logger)
	racePrepHandlers.Register(api)
	workouts.NewHandlers(workoutsSvc).Register(api)
	workoutfueling.NewHandlers(fuelingSvc).Register(api)
	workoutfuel.NewHandlers(workoutFuelSvc).Register(api)
	bodyweight.NewHandlers(bodyWeightSvc, cfg.DefaultUserTZ, logger).Register(api)
	energy.NewHandlers(energySvc, cfg.DefaultUserTZ).Register(api)
	dailyCtxSvc := dailycontext.NewService(
		summarySvc, hydrationRepo, workoutsRepo, workoutFuelRepo,
		bodyWeightRepo, goalsOverridesRepo, phasesRepo,
	)
	dailycontext.NewHandlers(dailyCtxSvc, cfg.DefaultUserTZ, logger).Register(api)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	listenErr := make(chan error, 1)
	go func() {
		logger.Info("http listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
			return
		}
		listenErr <- nil
	}()

	select {
	case err := <-listenErr:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown", "err", err)
	}
	return nil
}
