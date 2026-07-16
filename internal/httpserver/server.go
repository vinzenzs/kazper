// Package httpserver wires the Gin HTTP API. It was previously the cmd/api
// main package; it now lives behind the `kazper serve` subcommand.
package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	webapp "github.com/vinzenzs/kazper/apps/web"
	"github.com/vinzenzs/kazper/internal/achievements"
	"github.com/vinzenzs/kazper/internal/activitystreams"
	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/chat"
	"github.com/vinzenzs/kazper/internal/chatsessions"
	"github.com/vinzenzs/kazper/internal/coachcontext"
	"github.com/vinzenzs/kazper/internal/coachmemory"
	"github.com/vinzenzs/kazper/internal/config"
	"github.com/vinzenzs/kazper/internal/cookidoo"
	"github.com/vinzenzs/kazper/internal/dailycontext"
	"github.com/vinzenzs/kazper/internal/dailysummary"
	"github.com/vinzenzs/kazper/internal/devices"
	"github.com/vinzenzs/kazper/internal/effortanalytics"
	"github.com/vinzenzs/kazper/internal/energy"
	"github.com/vinzenzs/kazper/internal/expenditure"
	"github.com/vinzenzs/kazper/internal/fitnessmetrics"
	"github.com/vinzenzs/kazper/internal/garminauth"
	"github.com/vinzenzs/kazper/internal/garmincontrol"
	"github.com/vinzenzs/kazper/internal/garminsyncstatus"
	"github.com/vinzenzs/kazper/internal/gear"
	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/healthvitals"
	"github.com/vinzenzs/kazper/internal/hydration"
	"github.com/vinzenzs/kazper/internal/hydrationbalance"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/macrocycle"
	"github.com/vinzenzs/kazper/internal/mealplan"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/multisport"
	"github.com/vinzenzs/kazper/internal/off"
	"github.com/vinzenzs/kazper/internal/personalrecords"
	"github.com/vinzenzs/kazper/internal/pmc"
	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/publicfeed"
	"github.com/vinzenzs/kazper/internal/push"
	"github.com/vinzenzs/kazper/internal/racepacing"
	"github.com/vinzenzs/kazper/internal/raceprep"
	"github.com/vinzenzs/kazper/internal/races"
	"github.com/vinzenzs/kazper/internal/recoverymetrics"
	"github.com/vinzenzs/kazper/internal/shoppinglist"
	"github.com/vinzenzs/kazper/internal/store"
	"github.com/vinzenzs/kazper/internal/summary"
	"github.com/vinzenzs/kazper/internal/supplements"
	"github.com/vinzenzs/kazper/internal/trainingphases"
	"github.com/vinzenzs/kazper/internal/trainingplan"
	"github.com/vinzenzs/kazper/internal/vision"
	"github.com/vinzenzs/kazper/internal/wellness"
	"github.com/vinzenzs/kazper/internal/workoutcompliance"
	"github.com/vinzenzs/kazper/internal/workoutfuel"
	"github.com/vinzenzs/kazper/internal/workoutfueling"
	"github.com/vinzenzs/kazper/internal/workouts"
	"github.com/vinzenzs/kazper/internal/workoutstats"
	"github.com/vinzenzs/kazper/internal/workouttemplates"
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
		WebUser:     cfg.WebUser,
		WebPassword: cfg.WebPassword,
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
	// Macrocycle season container (add-macrocycle-planning). The repo doubles as
	// the phase service's macrocycle_id FK checker (interface-based, so
	// trainingphases doesn't import macrocycle — macrocycle reads training_phases
	// for its member list, which would otherwise cycle). The service is wired
	// below once racesRepo exists.
	macrocycleRepo := macrocycle.NewRepo(pool)
	phasesSvc.SetMacrocycleChecker(macrocycleRepo)
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
	workoutStatsSvc := workoutstats.NewService(workoutsRepo)
	effortAnalyticsSvc := effortanalytics.NewService(effortanalytics.NewRepo(pool))
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
	wellnessSvc := wellness.NewService(wellness.NewRepo(pool))
	supplementsSvc := supplements.NewService(supplements.NewRepo(pool))
	recoveryMetricsRepo := recoverymetrics.NewRepo(pool)
	recoveryMetricsSvc := recoverymetrics.NewService(recoveryMetricsRepo)
	dailySummaryRepo := dailysummary.NewRepo(pool)
	dailySummarySvc := dailysummary.NewService(dailySummaryRepo)
	gearRepo := gear.NewRepo(pool)
	gearSvc := gear.NewService(gearRepo)
	personalRecordsRepo := personalrecords.NewRepo(pool)
	personalRecordsSvc := personalrecords.NewService(personalRecordsRepo)
	coachMemoryRepo := coachmemory.NewRepo(pool)
	coachMemorySvc := coachmemory.NewService(coachMemoryRepo)
	athleteConfigRepo := athleteconfig.NewRepo(pool)
	athleteConfigSvc := athleteconfig.NewService(athleteConfigRepo, pool)
	// The effective-config provider resolves garmin-sourced fields against the
	// latest detection (manual otherwise). Every computational consumer reads
	// through it — one adapter here switches TSS derivation, zone resolution,
	// race pacing, and step compliance without per-package edits. With an empty
	// source policy it returns exactly the confirmed config (behavior-identical
	// to today until a source is flipped). Raw endpoints keep the raw repo.
	athleteConfigEffective := athleteconfig.NewEffectiveProvider(athleteConfigSvc)
	// Cross-inject the effective config so the workouts service derives a bike
	// workout's intensity_factor from the effective ftp_watts (mirrors the same
	// optional-setter convention as trainingPlanSvc.SetAthleteConfigRepo).
	workoutsSvc.SetAthleteConfigRepo(athleteConfigEffective)
	devicesSvc := devices.NewService(devices.NewRepo(pool))
	healthVitalsSvc := healthvitals.NewService(healthvitals.NewRepo(pool))
	achievementsSvc := achievements.NewService(achievements.NewRepo(pool))
	fitnessMetricsRepo := fitnessmetrics.NewRepo(pool)
	fitnessMetricsSvc := fitnessmetrics.NewService(fitnessMetricsRepo)
	hydrationBalanceRepo := hydrationbalance.NewRepo(pool)
	hydrationBalanceSvc := hydrationbalance.NewService(hydrationBalanceRepo)
	energySvc := energy.NewService(mealsRepo, workoutsRepo, bodyWeightRepo)
	// Adaptive expenditure reads intake from meals and the mass signal from the
	// body-weight capability's own trend (one smoothing truth) plus the raw
	// weigh-ins behind its gate — narrow reads, no writes, no goals coupling.
	expenditureSvc := expenditure.NewService(mealsRepo, bodyWeightSvc, bodyWeightRepo)
	// Protein-distribution needs to resolve weight at the queried date. Same
	// optional-setter pattern that meals/hydration use for SetWorkoutsRepo
	// (add-meal-workout-link).
	summarySvc.SetBodyWeightRepo(bodyWeightRepo)
	userTZ, err := time.LoadLocation(cfg.DefaultUserTZ)
	if err != nil {
		return err
	}
	racesRepo := races.NewRepo(pool)
	racesSvc := races.NewService(pool, racesRepo)
	// Macrocycle service needs the races repo for race_id FK validation; the
	// repo was created above (it backs the phase service's macrocycle checker).
	macrocycleSvc := macrocycle.NewService(macrocycleRepo, racesRepo)
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

	// Android push (opt-in, per add-garmin-relogin-push). The FCM sender is built
	// only when both FCM keys are configured; otherwise the service runs with a
	// nil sender (delivery is a no-op, token registration still persists). The
	// service is cross-injected below as the relogin notifier (sync-status) and
	// latch clearer (garmin-auth).
	var pushSender push.Sender
	if cfg.PushEnabled() {
		saJSON, saErr := cfg.FCMServiceAccount()
		if saErr != nil {
			return saErr
		}
		fcm, fcmErr := push.NewFCMClient(cfg.FCMProjectID, saJSON, 10*time.Second)
		if fcmErr != nil {
			return fcmErr
		}
		pushSender = fcm
	}
	pushSvc := push.NewService(push.NewRepo(pool), pushSender, logger)
	garminAuthSvc.SetReloginLatchClearer(pushSvc)
	garminAuthSvc.SetLogger(logger)

	cleanupCtx, cleanupCancel := context.WithCancel(ctx)
	defer cleanupCancel()
	go idempotency.RunCleanup(cleanupCtx, idempRepo, cfg.IdempotencyTTL, 15*time.Minute, logger)

	gin.SetMode(gin.ReleaseMode)
	r := BuildEngine()
	// Opt-in Prometheus metrics (default off). Registered before routes so the
	// middleware wraps every request; /metrics is a root infra route outside
	// bearer auth (sibling of /healthz) — enable only behind a private scrape.
	if cfg.MetricsEnabled {
		mtx := newMetrics()
		r.Use(mtx.middleware())
		r.GET("/metrics", gin.WrapH(mtx.handler()))
	}
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

	// Public race feed (public-race-feed): the ONLY unauthenticated data route.
	// Registered on the root engine, OUTSIDE auth.Middleware (sibling of
	// /healthz), gated by its own shared secret. Self-disables (503) when
	// FEED_SECRET is unset. Returns only a curated non-PII race projection.
	publicfeed.NewHandlers(
		publicfeed.NewService(publicfeed.NewRepo(pool), userTZ),
		cfg.FeedSecret,
	).Register(r)

	// Domain endpoints are versioned under /api/v1 (per add-api-versioning); the
	// infra endpoints above (/healthz, /readyz, /swagger) stay at root, unversioned.
	// config.APIBasePath is the shared source of truth (also used by the MCP client
	// default and the chat loopback dispatcher).
	api := r.Group(config.APIBasePath)
	// Streaming/long routes are exempt from the per-request deadline (they own
	// their own budgets); self-capped upload routes are exempt from the global
	// body cap. Prefixes are full /api/v1 paths.
	base := config.APIBasePath
	timeoutExempt := []string{base + "/chat", base + "/meals/from_photo", base + "/garmin"}
	bodyExempt := []string{base + "/meals/from_photo", base + "/garmin"}
	api.Use(requestIDMiddleware())
	api.Use(requestLogger(logger))
	api.Use(requestTimeout(cfg.HTTPRequestTimeout, timeoutExempt))
	api.Use(bodyLimit(cfg.MaxRequestBodyBytes, bodyExempt))
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
	macrocycle.NewHandlers(macrocycleSvc).Register(api)
	hydration.NewHandlers(hydrationSvc).Register(api)
	hydration.NewSummaryHandlers(hydrationSvc, cfg.DefaultUserTZ, logger).Register(api)
	racePrepHandlers := raceprep.NewHandlers(racePrepSvc)
	racePrepHandlers.SetLogger(logger)
	racePrepHandlers.Register(api)
	races.NewHandlers(racesSvc).Register(api)
	// Per-leg race pacing plan (add-race-pacing-plan): compute-on-read power/pace
	// bands from athlete-config thresholds over the race/leg tables, plus
	// persisted per-leg overrides. Multi-repo aggregator (races + athlete-config).
	racepacing.NewHandlers(
		racepacing.NewService(racesRepo, athleteConfigEffective, racepacing.NewRepo(pool)),
	).Register(api)
	mealplan.NewHandlers(mealPlanSvc).Register(api)
	shoppinglist.NewHandlers(shoppingSvc).Register(api)
	workouts.NewHandlers(workoutsSvc).Register(api)
	workoutstats.NewHandlers(workoutStatsSvc, cfg.DefaultUserTZ, logger).Register(api)
	effortanalytics.NewHandlers(effortAnalyticsSvc, latestBodyWeight{repo: bodyWeightRepo}, cfg.DefaultUserTZ, logger).Register(api)
	// Raw activity streams (persist-activity-streams): stores the 1 Hz series,
	// delegates the best-effort ladder to effort-analytics, and derives the
	// workout's execution metrics (VI/EF/decoupling).
	activitystreams.NewHandlers(
		activitystreams.NewService(activitystreams.NewRepo(pool), workoutsRepo, effortAnalyticsSvc),
	).Register(api)
	// Performance Management Chart (add-performance-management): compute-on-read
	// CTL/ATL/TSB over completed-workout TSS. Read-only; own read repo.
	pmcSvc := pmc.NewService(pmc.NewRepo(pool))
	pmc.NewHandlers(pmcSvc, macroResolver{repo: macrocycleRepo}, cfg.DefaultUserTZ, logger).Register(api)
	// Cross-inject the PMC series into wellness for the correlation read.
	wellnessSvc.SetPMCProvider(pmcWellnessAdapter{svc: pmcSvc})
	wellness.NewHandlers(wellnessSvc).Register(api)
	supplements.NewHandlers(supplementsSvc).Register(api)
	workoutTemplatesRepo := workouttemplates.NewRepo(pool)
	workouttemplates.NewHandlers(workouttemplates.NewService(workoutTemplatesRepo)).Register(api)
	multisportRepo := multisport.NewRepo(pool)
	multisport.NewHandlers(multisport.NewService(multisportRepo)).Register(api)
	trainingPlanSvc := trainingplan.NewService(trainingplan.NewRepo(pool), pool, workoutsRepo, workoutTemplatesRepo, cfg.DefaultUserTZ)
	// Cross-inject the effective config so EffectiveProgram resolves zone-reference
	// targets into absolute power_w/hr_bpm ranges against the effective zones
	// (mirrors SetWorkoutsRepo).
	trainingPlanSvc.SetAthleteConfigRepo(athleteConfigEffective)
	// Cross-inject the multisport-template repo so EffectiveProgram resolves a
	// multisport workout's per-segment programs (each by its own sport).
	trainingPlanSvc.SetMultisportRepo(multisportRepo)
	trainingplan.NewHandlers(trainingPlanSvc).Register(api)
	// Per-step execution compliance (add-step-compliance): compares a completed
	// workout's splits against its effective program. Leaf package (no repo),
	// wired after trainingPlanSvc since it is the program provider.
	workoutcompliance.NewHandlers(workoutcompliance.NewService(workoutsRepo, trainingPlanSvc)).Register(api)
	workoutfueling.NewHandlers(fuelingSvc).Register(api)
	workoutfuel.NewHandlers(workoutFuelSvc).Register(api)
	bodyweight.NewHandlers(bodyWeightSvc, cfg.DefaultUserTZ, logger).Register(api)
	recoverymetrics.NewHandlers(recoveryMetricsSvc).Register(api)
	dailysummary.NewHandlers(dailySummarySvc).Register(api)
	gear.NewHandlers(gearSvc).Register(api)
	personalrecords.NewHandlers(personalRecordsSvc).Register(api)
	coachmemory.NewHandlers(coachMemorySvc, cfg.DefaultUserTZ, logger).Register(api)
	athleteconfig.NewHandlers(athleteConfigSvc).Register(api)
	devices.NewHandlers(devicesSvc).Register(api)
	healthvitals.NewHandlers(healthVitalsSvc).Register(api)
	achievements.NewHandlers(achievementsSvc).Register(api)
	fitnessmetrics.NewHandlers(fitnessMetricsSvc).Register(api)
	hydrationbalance.NewHandlers(hydrationBalanceSvc).Register(api)
	garminauth.NewHandlers(garminAuthSvc, garminEnabled).Register(api)
	// Garmin login proxy (per add-garmin-mcp-login) + scheduling orchestration
	// (per add-garmin-scheduling): forwards login to the bridge at
	// GARMIN_BRIDGE_URL and compiles/schedules planned workouts onto the watch.
	// Empty URL ⇒ the endpoints return 503 garmin_disabled.
	garminControl := garmincontrol.NewHandlers(cfg.GarminBridgeURL)
	garminControl.SetSchedulingDeps(workoutsRepo, workoutTemplatesRepo, trainingPlanSvc)
	garminControl.SetMultisportRepo(multisportRepo)
	garminControl.Register(api)
	// Device push-token registration (per add-garmin-relogin-push): mobile-only;
	// works whether or not FCM is configured so a device can register ahead of
	// push being enabled.
	push.NewHandlers(pushSvc).Register(api)
	// Garmin sync-run log (per add-garmin-connect-and-sync-status): the bridge
	// records each /sync run (garmin identity only); the app + coach read
	// /garmin/sync-status. Gated 503 garmin_disabled when GARMIN_API_TOKEN is unset.
	// Cross-inject the push notifier + Garmin-token presence so an error-close
	// with an absent token fires a relogin push and a success-close clears it.
	syncStatusSvc := garminsyncstatus.NewService(garminsyncstatus.NewRepo(pool))
	syncStatusSvc.SetReloginNotifier(pushSvc)
	syncStatusSvc.SetGarminTokenPresence(garminAuthSvc)
	syncStatusSvc.SetLogger(logger)
	garminsyncstatus.NewHandlers(syncStatusSvc, garminEnabled).Register(api)
	energy.NewHandlers(energySvc, cfg.DefaultUserTZ).Register(api)
	expenditure.NewHandlers(expenditureSvc, cfg.DefaultUserTZ, logger).Register(api)
	dailyCtxSvc := dailycontext.NewService(
		summarySvc, hydrationRepo, workoutsRepo, workoutFuelRepo,
		bodyWeightRepo, goalsOverridesRepo, phasesRepo,
		recoveryMetricsRepo, fitnessMetricsRepo, hydrationBalanceRepo,
		coachMemoryRepo, wellness.NewRepo(pool), supplements.NewRepo(pool),
	)
	dailycontext.NewHandlers(dailyCtxSvc, cfg.DefaultUserTZ, logger).Register(api)
	coachCtxSvc := coachcontext.NewService(workoutsRepo, fitnessMetricsRepo, recoveryMetricsRepo, phasesRepo, athleteConfigRepo, bodyWeightRepo)
	// Cross-inject multisport so the recent-load by_sport summary decomposes a
	// multisport workout into its segment sports (multisport-phase-3).
	coachCtxSvc.SetMultisportRepo(multisportRepo)
	// Cross-inject the macrocycle repo so /context/training surfaces the season
	// covering the anchor date + the current period's position (add-macrocycle-planning).
	coachCtxSvc.SetMacrocycleRepo(macrocycleRepo)
	// Cross-inject coach memory so /context/training folds in active standing
	// items + window-scoped recommendations (widen-coach-recs-to-memory).
	coachCtxSvc.SetMemoryRepo(coachMemoryRepo)
	// Cross-inject the athlete-config service so /context/training carries the
	// garmin detection, source policy, and effective config beside the confirmed
	// config (separate-garmin-threshold-detection).
	coachCtxSvc.SetAthleteConfigService(athleteConfigSvc)
	coachcontext.NewHandlers(coachCtxSvc, cfg.DefaultUserTZ, logger).Register(api)
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

	// Serve the embedded coach-dashboard SPA at / (per add-coach-dashboard).
	// Registered last so it only catches paths that fall through to NoRoute —
	// the /api/v1 group and infra endpoints above always win, and unknown
	// /api/v1 paths keep the JSON 404 contract.
	dist, distErr := webapp.DistFS()
	if distErr != nil {
		return distErr
	}
	if err := RegisterSPA(r, dist, authCfg); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		// Bound slow-loris header/body reads and idle keep-alives. WriteTimeout is
		// deliberately left unset so SSE streams (/chat) stay open — per-request
		// deadlines are handled by the requestTimeout middleware instead.
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 120 * time.Second,
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
