package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mostransport/vsm-trainer/internal/config"
	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/handler"
	"github.com/mostransport/vsm-trainer/internal/llm"
	appmiddleware "github.com/mostransport/vsm-trainer/internal/middleware"
	"github.com/mostransport/vsm-trainer/internal/push"
	"github.com/mostransport/vsm-trainer/internal/repo/postgres"
	"github.com/mostransport/vsm-trainer/internal/service"
	"github.com/mostransport/vsm-trainer/internal/simulation"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	catalog, err := content.Load()
	if err != nil {
		return fmt.Errorf("load content: %w", err)
	}
	wagonClasses, err := content.LoadWagonClasses()
	if err != nil {
		return fmt.Errorf("load wagon classes: %w", err)
	}
	wagonLevels, err := content.LoadLevels()
	if err != nil {
		return fmt.Errorf("load wagon levels: %w", err)
	}
	curriculum, err := content.LoadCurriculum()
	if err != nil {
		return fmt.Errorf("load curriculum: %w", err)
	}
	simTemplate, err := simulation.Load()
	if err != nil {
		return fmt.Errorf("load simulation template: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	if cfg.AdminBootstrapEmail != "" && cfg.AdminBootstrapPassword != "" {
		if admin, err := store.BootstrapAdmin(ctx, cfg.AdminBootstrapEmail, cfg.AdminBootstrapPassword); err != nil {
			slog.Error("admin bootstrap failed", "email", cfg.AdminBootstrapEmail, "error", err)
		} else {
			slog.Info("admin bootstrap ready", "email", admin.Email)
		}
	}

	var client llm.LLMClient
	switch cfg.LLMMode {
	case "mock":
		client = llm.NewMockLLM()
	case "gigachat":
		if cfg.GigaChatClientID == "" || cfg.GigaChatClientSecret == "" {
			return fmt.Errorf("GIGACHAT_CLIENT_ID and GIGACHAT_CLIENT_SECRET are required")
		}
		client = llm.NewGigaChatClient(cfg.GigaChatAuthURL, cfg.GigaChatAPIURL, cfg.GigaChatModel,
			cfg.GigaChatClientID, cfg.GigaChatClientSecret, cfg.GigaChatInsecure)
	default:
		return fmt.Errorf("unsupported LLM_MODE %q", cfg.LLMMode)
	}

	auth := service.NewAuthService(store, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	situations := service.NewSituationService(store, client, catalog)
	simulationService := service.NewSimulationService(store, simTemplate, cfg.PointsNamespace, client)
	wagonCatalog := catalog
	wagonManager := service.NewWagonManager(store, wagonCatalog, wagonLevels)
	defer wagonManager.StopAll()
	if err := wagonManager.Recover(ctx, wagonClasses); err != nil {
		return fmt.Errorf("recover wagon sessions: %w", err)
	}
	wagonService := service.NewWagonService(store, wagonCatalog, wagonClasses, wagonLevels, wagonManager)
	prizeService := service.NewPrizeService(store)
	// The default Sender just logs -- real Expo/FCM delivery is blocked on
	// the mobile team's credentials and expo-notifications setup (see
	// internal/push's package doc). Swapping it for a real implementation
	// later is a one-file follow-up, selected here the same way LLM_MODE
	// selects llm.LLMClient above, once there's more than one Sender.
	pushSender := push.NewLogSender()
	pushService := service.NewPushService(store, pushSender)
	learningService := service.NewLearningService(store, wagonCatalog, wagonClasses, curriculum, wagonService, wagonManager, prizeService, pushService)
	h := &handler.Handlers{
		Auth:         auth,
		Profile:      service.NewProfileService(store, cfg.PointsNamespace),
		Session:      service.NewSessionService(store, catalog, cfg.SituationsPerSession, situations, cfg.PointsNamespace),
		Situation:    situations,
		Admin:        service.NewAdminService(store, catalog, wagonLevels, curriculum, prizeService, cfg.PointsNamespace),
		Simulation:   simulationService,
		Wagon:        wagonService,
		WagonManager: wagonManager,
		WagonClasses: wagonClasses,
		Learning:     learningService,
		Prize:        prizeService,
		Push:         pushService,
	}
	router := routes(h, auth, store)
	server := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := situations.CloseExpired(ctx); err != nil {
					slog.Error("timer closer failed", "error", err)
				}
				if err := simulationService.CloseExpired(ctx); err != nil {
					slog.Error("simulation timer closer failed", "error", err)
				}
			}
		}
	}()
	// Prize-credit expiry runs on its own, much longer-interval ticker in a
	// separate goroutine: checking a 24h expiry window every 2 seconds (the
	// situations/simulation timer loop's cadence) would be wasteful, and a
	// separate ticker/goroutine keeps the two cadences independent instead
	// of one loop juggling two intervals.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := pushService.SweepExpiringPrizeCredits(ctx); err != nil {
					slog.Error("prize expiry push sweep failed", "error", err)
				}
			}
		}
	}()
	slog.Info("server listening", "port", cfg.ServerPort, "scenarios", len(catalog.Scenarios), "passengers", len(catalog.Passengers))
	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func routes(h *handler.Handlers, auth *service.AuthService, store *postgres.Store) http.Handler {
	r := chi.NewRouter()
	r.Use(localWebCORS)
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Use(requestLogger)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{"llm_errors": service.LLMErrors.Load()})
	})
	r.Route("/auth", func(r chi.Router) {
		r.Use(appmiddleware.AuthRateLimit(10, time.Minute))
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
	})
	r.Route("/api", func(r chi.Router) {
		r.Use(appmiddleware.JWTAuth(auth))
		r.Get("/profile", h.GetProfile)
		r.Get("/notifications", h.GetNotifications)
		r.Get("/challenges/weekly", h.GetChallenge)
		r.Get("/leaderboard", h.GetLeaderboard)
		r.Get("/leaderboards", h.GetScopedLeaderboard)
		r.Post("/session/start", h.StartSession)
		r.Post("/session/simulations", h.StartSimulation)
		r.Post("/session/wagon/start", h.StartWagonSession)
		r.Get("/wagon/classes", h.ListWagonClasses)
		r.Get("/wagon/levels", h.ListWagonLevels)
		r.Get("/session/simulations/{id}", h.GetSimulation)
		r.Get("/session/simulations/{id}/result", h.SimulationResult)
		r.Post("/session/simulations/{id}/actions", h.SimulationAction)
		r.Post("/session/simulations/{id}/dialogue", h.SimulationDialogue)
		r.Get("/session/{id}", h.GetSession)
		r.Post("/session/{id}/finish", h.FinishSession)
		r.Get("/situation/{id}", h.GetSituation)
		r.Post("/situation/{id}/message", h.SendMessage)
		r.Post("/situation/{id}/escalate", h.Escalate)
		r.Post("/situation/{id}/finish", h.FinishSituation)
		r.Get("/learning/map", h.GetLearningMap)
		r.Get("/learning/lessons/{id}", h.GetLesson)
		r.Post("/learning/lessons/{id}/answers", h.SubmitLessonAnswer)
		r.Post("/learning/lessons/{id}/practice", h.StartLessonPractice)
		r.Post("/learning/lessons/{id}/finalize", h.FinalizeLessonPractice)
		r.Get("/me/learning", h.GetMyLearning)
		r.Post("/me/push-subscriptions", h.RegisterPushSubscription)
	})
	r.Get("/api/wagon/{id}/ws", h.WagonWS)
	r.Route("/admin", func(r chi.Router) {
		r.Use(appmiddleware.AdminAuth(auth))
		r.Post("/users", h.CreateExternalUser)
		r.Post("/players", h.CreatePlayerAccount)
		r.Get("/users/{id}/learning-summary", h.LearningSummary)
		r.Post("/users/{id}/prize-credits/demo-seed", h.SeedDemoPrizeEntry)
	})
	return r
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrapped, r)
		slog.Info("http request", "request_id", middleware.GetReqID(r.Context()),
			"method", r.Method, "path", r.URL.Path, "status", wrapped.Status(),
			"duration_ms", time.Since(started).Milliseconds())
	})
}

// localWebCORS permits only configured browser origins.
func localWebCORS(next http.Handler) http.Handler {
	allowed := os.Getenv("CORS_ORIGINS")
	if allowed == "" {
		allowed = "http://localhost:8081,http://localhost:19006,http://127.0.0.1:8081,http://127.0.0.1:19006"
	}
	origins := map[string]bool{}
	for _, origin := range strings.Split(allowed, ",") {
		origins[strings.TrimSpace(origin)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		} else if r.Method == http.MethodOptions && origin != "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
