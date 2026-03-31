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
	printBanner() // Welcome developers!

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

	// --- Initialize Native Connector Registry ---
	// TODO: Get email config from the environment/cfg in a future batch
	registry := connectors.NewRegistry(nil)

	eng := engine.New(repo, cfg.MasterKey, fwd, registry)
	admService := admin.NewService(repo, cfg.MasterKey)

	// Phase 1: Initialize Queue and Worker Pool
	queue := engine.NewMemoryQueue(1024)
	workerPool := engine.NewWorkerPool(queue, eng, 4) // Configurable worker count?
	workerPool.Start(context.Background())

	wh := httpapi.NewWebhookHandler(eng, queue)
	adminH := httpapi.NewAdminHandler(admService, eng)
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

	slog.Info("Server listening", "port", cfg.Port)
	fmt.Printf("\n🚀 Graft Webhook Bridge successfully started!\n\n")
	fmt.Printf("   Dashboard & Health :\t http://localhost:%s/\n", cfg.Port)
	fmt.Printf("   Admin API Base     :\t http://localhost:%s/api/v1/\n", cfg.Port)
	fmt.Printf("   Webhook Listen Path:\t http://localhost:%s/hook/{path}\n\n", cfg.Port)
	fmt.Printf("Use highly secure keys in production!\n\n")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	workerPool.Stop()

	slog.Info("Server exited gracefully")
	return nil
}

// printBanner displays the application's logo and branding.
func printBanner() {
	banner := `
   _____            __ _   
  / ____|          / _| |  
 | |  __ _ __ __ _| |_| |_ 
 | | |_ | '__/ _` + "`" + ` |  _| __|
 | |__| | | | (_| | | | |_ 
  \_____|_|  \__,_|_|  \__|
                           
  A Self-Hosted Webhook Bridge
================================
`
	fmt.Print(banner)
}
