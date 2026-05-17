package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/DongDuong2001/graft/internal/config"
)

// HTTPServer wraps net/http.Server with Graft defaults from config.
type HTTPServer struct {
	srv  *http.Server
	addr string
	tls  *tls.Config
}

// NewHTTPServer builds an HTTP server listening on cfg.Port.
func NewHTTPServer(cfg config.Config, handler http.Handler, tlsConfig *tls.Config) *HTTPServer {
	addr := ":" + cfg.Port
	return &HTTPServer{
		addr: addr,
		tls:  tlsConfig,
		srv: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			TLSConfig:         tlsConfig,
		},
	}
}

// Addr returns the listen address (e.g. ":8080").
func (s *HTTPServer) Addr() string {
	return s.srv.Addr
}

// IsTLS returns true if the server is configured for TLS.
func (s *HTTPServer) IsTLS() bool {
	return s.tls != nil
}

// ListenAndServe starts the server (blocks).
func (s *HTTPServer) ListenAndServe() error {
	if s.tls != nil {
		if err := s.srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("https server: %w", err)
		}
	} else {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server: %w", err)
		}
	}
	return nil
}

// Shutdown gracefully stops accepting new connections.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
