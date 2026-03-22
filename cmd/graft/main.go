package main

import (
	"log/slog"
	"os"

	"Graft/internal/app"
)

func main() {
	// Use structured JSON logging for production-ready output
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Starting Graft Webhook Bridge...")
	if err := app.Run(); err != nil {
		slog.Error("Application failed", "error", err)
		os.Exit(1)
	}
}
