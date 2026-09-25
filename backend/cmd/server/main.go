package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mostransport/vsm-trainer/internal/config"
	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/handler"
	"github.com/mostransport/vsm-trainer/internal/llm"
	appmiddleware "github.com/mostransport/vsm-trainer/internal/middleware"
	"github.com/mostransport/vsm-trainer/internal/repo/postgres"
	"github.com/mostransport/vsm-trainer/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	catalog, err := content.Load()
	if err != nil {
		return fmt.Errorf("load content: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

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
	h := &handler.Handlers{
		Auth:      auth,
		Profile:   service.NewProfileService(store),
		Session:   service.NewSessionService(store, catalog, cfg.SituationsPerSession, situations),
		Situation: situations,
	}
	router := routes(h, auth)
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

func routes(h *handler.Handlers, auth *service.AuthService) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	r.Post("/auth/register", h.Register)
	r.Post("/auth/login", h.Login)
	r.Post("/auth/refresh", h.Refresh)
	r.Route("/api", func(r chi.Router) {
		r.Use(appmiddleware.JWTAuth(auth))
		r.Get("/profile", h.GetProfile)
		r.Get("/leaderboard", h.GetLeaderboard)
		r.Post("/session/start", h.StartSession)
		r.Get("/session/{id}", h.GetSession)
		r.Post("/session/{id}/finish", h.FinishSession)
		r.Get("/situation/{id}", h.GetSituation)
		r.Post("/situation/{id}/message", h.SendMessage)
		r.Post("/situation/{id}/escalate", h.Escalate)
		r.Post("/situation/{id}/finish", h.FinishSituation)
	})
	return r
}
