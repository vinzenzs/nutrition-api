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
	"github.com/vinzenzs/nutrition-api/internal/chat"
	"github.com/vinzenzs/nutrition-api/internal/chatsessions"
	"github.com/vinzenzs/nutrition-api/internal/config"
	"github.com/vinzenzs/nutrition-api/internal/cookidoo"
	"github.com/vinzenzs/nutrition-api/internal/dailycontext"
	"github.com/vinzenzs/nutrition-api/internal/energy"
	"github.com/vinzenzs/nutrition-api/internal/fitnessmetrics"
	"github.com/vinzenzs/nutrition-api/internal/garminauth"
	"github.com/vinzenzs/nutrition-api/internal/garmincontrol"
	"github.com/vinzenzs/nutrition-api/internal/goals"
	"github.com/vinzenzs/nutrition-api/internal/hydration"
	"github.com/vinzenzs/nutrition-api/internal/hydrationbalance"
	"github.com/vinzenzs/nutrition-api/internal/idempotency"
	"github.com/vinzenzs/nutrition-api/internal/mealplan"
	"github.com/vinzenzs/nutrition-api/internal/meals"
	"github.com/vinzenzs/nutrition-api/internal/off"
	"github.com/vinzenzs/nutrition-api/internal/products"
	"github.com/vinzenzs/nutrition-api/internal/raceprep"
	"github.com/vinzenzs/nutrition-api/internal/races"
	"github.com/vinzenzs/nutrition-api/internal/dailysummary"
	"github.com/vinzenzs/nutrition-api/internal/recoverymetrics"
	"github.com/vinzenzs/nutrition-api/internal/shoppinglist"
	"github.com/vinzenzs/nutrition-api/internal/store"
	"github.com/vinzenzs/nutrition-api/internal/summary"
	"github.com/vinzenzs/nutrition-api/internal/trainingphases"
	"github.com/vinzenzs/nutrition-api/internal/trainingplan"
	"github.com/vinzenzs/nutrition-api/internal/vision"
	"github.com/vinzenzs/nutrition-api/internal/workoutfuel"
	"github.com/vinzenzs/nutrition-api/internal/workoutfueling"
	"github.com/vinzenzs/nutrition-api/internal/workouts"
	"github.com/vinzenzs/nutrition-api/internal/workouttemplates"
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
	authCfg := auth.Config{
		MobileToken: cfg.MobileToken,
		AgentToken:  cfg.AgentToken,
		GarminToken: cfg.GarminToken,
	}
	if err := authCfg.Validate(); err != nil {
		return err
	}

	// Garmin integration is opt-in: only decode the enc key when the dedicated
	// token is set. ValidateForServe already verified the key shape.
	garminEnabled := cfg.GarminToken != ""
	var garminEncKey []byte
	if garminEnabled {
		k, err := cfg.GarminEncKey()
		if err != nil {
			return err
		}
		garminEncKey = k
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
	// Server-side Cookidoo recipe import: fetch + JSON-LD parse. Always wired
	// (no API key required); only the per-request timeout is configurable.
	productsSvc.SetCookidooClient(cookidoo.New(cookidoo.Config{Timeout: cfg.CookidooTimeout}))
	mealsRepo := meals.NewRepo(pool)
	mealsSvc := meals.NewService(pool, mealsRepo, productsRepo)

	// Vision (Claude) is optional: when ANTHROPIC_API_KEY is unset, leave the
	// client nil so /meals/from_photo can return 503 vision_unavailable
	// without blowing up the rest of the API.
	var visionClient *vision.Client
	if cfg.AnthropicAPIKey != "" {
		vc, err := vision.New(vision.Config{
			APIKey:  cfg.AnthropicAPIKey,
			Model:   cfg.ClaudeVisionModel,
			Timeout: cfg.VisionTimeout,
		})
		if err != nil {
			return err
		}
		visionClient = vc
	}

	// Nutrition chat (POST /chat) is optional the same way: when
	// ANTHROPIC_API_KEY is unset, chatSvc stays nil and the handler returns 503
	// chat_unavailable. The loopback handler (the engine) is wired after the
	// engine is built, below.
	var chatSvc *chat.Service
	if cfg.AnthropicAPIKey != "" {
		cs, err := chat.New(cfg.AnthropicAPIKey, chat.Config{
			Model:              cfg.ChatModel,
			MaxToolRounds:      cfg.ChatMaxToolRounds,
			MaxHistoryMessages: cfg.ChatMaxHistoryMessages,
			RequestTimeout:     cfg.ChatRequestTimeout,
			DietaryPreferences: cfg.ChatDietaryPreferences,
			Timezone:           cfg.DefaultUserTZ,
		})
		if err != nil {
			return err
		}
		chatSvc = cs
	}
	// Chat-session persistence is independent of the API key: the CRUD surface
	// (and the store the loop writes into) works even when chatSvc is nil — only
	// POST /chat itself returns 503 without a key.
	chatSessionsRepo := chatsessions.NewRepo(pool)
	chatSessionsSvc := chatsessions.NewService(chatSessionsRepo)
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
	workoutsSvc := workouts.NewService(workoutsRepo, pool, cfg.DefaultUserTZ)
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
	recoveryMetricsRepo := recoverymetrics.NewRepo(pool)
	recoveryMetricsSvc := recoverymetrics.NewService(recoveryMetricsRepo)
	dailySummaryRepo := dailysummary.NewRepo(pool)
	dailySummarySvc := dailysummary.NewService(dailySummaryRepo)
	fitnessMetricsRepo := fitnessmetrics.NewRepo(pool)
	fitnessMetricsSvc := fitnessmetrics.NewService(fitnessMetricsRepo)
	hydrationBalanceRepo := hydrationbalance.NewRepo(pool)
	hydrationBalanceSvc := hydrationbalance.NewService(hydrationBalanceRepo)
	energySvc := energy.NewService(mealsRepo, workoutsRepo, bodyWeightRepo)
	// Protein-distribution needs to resolve weight at the queried date. Same
	// optional-setter pattern that meals/hydration use for SetWorkoutsRepo
	// (add-meal-workout-link).
	summarySvc.SetBodyWeightRepo(bodyWeightRepo)
	userTZ, err := time.LoadLocation(cfg.DefaultUserTZ)
	if err != nil {
		return err
	}
	racesSvc := races.NewService(pool, races.NewRepo(pool))
	// Meal plan: planned-meal CRUD + the eaten transition. Cross-inject the
	// products repo (FK validation, mirroring mealsSvc.SetWorkoutsRepo) and the
	// meals service (the eaten transition logs a real meal entry atomically).
	mealPlanSvc := mealplan.NewService(pool, mealplan.NewRepo(pool))
	mealPlanSvc.SetProductsRepo(productsRepo)
	mealPlanSvc.SetMealsService(mealsSvc)
	// Shopping list: dumb checklist with soft product provenance. Inject the
	// products repo for the recipe_product_id existence check only.
	shoppingSvc := shoppinglist.NewService(shoppinglist.NewRepo(pool))
	shoppingSvc.SetProductsRepo(productsRepo)
	racePrepSvc := raceprep.NewService(time.Now, userTZ, pool)
	// recommend-workout-fuel needs the workouts row + body-weight resolver.
	// Optional setters so the existing constructor signature stays stable
	// (same convention meals/hydration use for SetWorkoutsRepo from
	// add-meal-workout-link).
	racePrepSvc.SetWorkoutsRepo(workoutsRepo)
	racePrepSvc.SetBodyWeightRepo(bodyWeightRepo)
	idempRepo := idempotency.NewRepo(pool)

	// Garmin token store (encrypted single-row blob). The service is wired with
	// the enc key only when the integration is enabled; the handler short-
	// circuits 503 garmin_disabled otherwise.
	garminAuthSvc, err := garminauth.NewService(garminauth.NewRepo(pool), garminEncKey)
	if err != nil {
		return err
	}

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
	mealsHandlers := meals.NewHandlers(mealsSvc)
	mealsHandlers.SetVision(visionClient, cfg.MealFromPhotoMaxBytes)
	mealsHandlers.Register(api)
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
	races.NewHandlers(racesSvc).Register(api)
	mealplan.NewHandlers(mealPlanSvc).Register(api)
	shoppinglist.NewHandlers(shoppingSvc).Register(api)
	workouts.NewHandlers(workoutsSvc).Register(api)
	workoutTemplatesRepo := workouttemplates.NewRepo(pool)
	workouttemplates.NewHandlers(workouttemplates.NewService(workoutTemplatesRepo)).Register(api)
	trainingPlanSvc := trainingplan.NewService(trainingplan.NewRepo(pool), pool, workoutsRepo, workoutTemplatesRepo, cfg.DefaultUserTZ)
	trainingplan.NewHandlers(trainingPlanSvc).Register(api)
	workoutfueling.NewHandlers(fuelingSvc).Register(api)
	workoutfuel.NewHandlers(workoutFuelSvc).Register(api)
	bodyweight.NewHandlers(bodyWeightSvc, cfg.DefaultUserTZ, logger).Register(api)
	recoverymetrics.NewHandlers(recoveryMetricsSvc).Register(api)
	dailysummary.NewHandlers(dailySummarySvc).Register(api)
	fitnessmetrics.NewHandlers(fitnessMetricsSvc).Register(api)
	hydrationbalance.NewHandlers(hydrationBalanceSvc).Register(api)
	garminauth.NewHandlers(garminAuthSvc, garminEnabled).Register(api)
	// Garmin login proxy (per add-garmin-mcp-login) + scheduling orchestration
	// (per add-garmin-scheduling): forwards login to the bridge at
	// GARMIN_BRIDGE_URL and compiles/schedules planned workouts onto the watch.
	// Empty URL ⇒ the endpoints return 503 garmin_disabled.
	garminControl := garmincontrol.NewHandlers(cfg.GarminBridgeURL)
	garminControl.SetSchedulingDeps(workoutsRepo, workoutTemplatesRepo, trainingPlanSvc)
	garminControl.Register(api)
	energy.NewHandlers(energySvc, cfg.DefaultUserTZ).Register(api)
	dailyCtxSvc := dailycontext.NewService(
		summarySvc, hydrationRepo, workoutsRepo, workoutFuelRepo,
		bodyWeightRepo, goalsOverridesRepo, phasesRepo,
		recoveryMetricsRepo, fitnessMetricsRepo, hydrationBalanceRepo,
	)
	dailycontext.NewHandlers(dailyCtxSvc, cfg.DefaultUserTZ, logger).Register(api)
	// POST /chat streams SSE. The idempotency middleware is a no-op here: it only
	// engages when an Idempotency-Key header is present, and the chat client does
	// not send one (a streamed conversation turn is not a replayable write — the
	// idempotency it needs lives one level down, on the individual tool calls the
	// loop dispatches, each of which carries its own derived key).
	chat.NewHandlers(chatSvc).Register(api)
	chatsessions.NewHandlers(chatSessionsSvc).Register(api)

	// The chat loop dispatches tools as in-process HTTP calls back through this
	// same engine (full auth + idempotency + logging middleware). Wire the
	// loopback target now that every route — including /chat itself — is
	// registered. Guarded because chatSvc is nil when no API key is configured.
	// The session store is injected here too: the loop loads history from and
	// persists turns into chat_sessions / chat_messages.
	if chatSvc != nil {
		chatSvc.SetLoopbackHandler(r)
		chatSvc.SetSessionStore(chatSessionStore{repo: chatSessionsRepo})
	}

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
