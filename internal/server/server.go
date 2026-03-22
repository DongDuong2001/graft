package server

import (
	"context"
	"fmt"
	"net/http"

	"Graft/internal/config"
)

// HTTPServer wraps net/http.Server with Graft defaults from config.
type HTTPServer struct {
	srv *http.Server
}

// NewHTTPServer builds an HTTP server listening on cfg.Port.
func NewHTTPServer(cfg config.Config, handler http.Handler) *HTTPServer {
	addr := ":" + cfg.Port
	return &HTTPServer{
		srv: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
	}
}

// Addr returns the listen address (e.g. ":8080").
func (s *HTTPServer) Addr() string {
	return s.srv.Addr
}

// ListenAndServe starts the server (blocks).
func (s *HTTPServer) ListenAndServe() error {
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

// Shutdown gracefully stops accepting new connections.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
