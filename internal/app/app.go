package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"Graft/internal/admin"
	"Graft/internal/config"
	"Graft/internal/connectors"
	"Graft/internal/engine"
	"Graft/internal/forwarder"
	"Graft/internal/httpapi"
	"Graft/internal/middleware"
	"Graft/internal/router"
	"Graft/internal/server"
	"Graft/internal/storage"
)

// Run loads configuration, wires dependencies, and blocks serving HTTP.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	repo, err := storage.NewSQLiteRepo(db, cfg.MasterKey)
	if err != nil {
		return fmt.Errorf("repository: %w", err)
	}

	httpFwd := connectors.NewHTTPForwarder(connectors.HTTPConfig{
		Timeout:       cfg.ForwardTimeout,
		MaxRetries:    cfg.ForwardMaxRetries,
		BaseRetryWait: cfg.ForwardRetryBase,
	})
	fwd := forwarder.NewSyncForwarder(httpFwd)

	eng := engine.New(repo, cfg.MasterKey, fwd)
	admService := admin.NewService(repo, cfg.MasterKey)

	wh := httpapi.NewWebhookHandler(eng)
	adminH := httpapi.NewAdminHandler(admService)
	adminMux := http.NewServeMux()
	adminH.Register(adminMux)

	rl := middleware.NewFixedWindowLimiter(cfg.RateLimitMax, cfg.RateLimitWindow, cfg.TrustForwardedHeaders)

	root := router.NewRootMux(router.Config{
		WebhookHandler: wh,
		AdminInner:     adminMux,
		AdminAPIKey:    cfg.AdminAPIKey,
		RateLimiter:    rl,
	})

	srv := server.NewHTTPServer(cfg, root)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to listen", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("Server listening", "port", cfg.Port, "admin_api", "/api/v1/", "webhook_prefix", "/hook/")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	slog.Info("Server exited gracefully")
	return nil
}
