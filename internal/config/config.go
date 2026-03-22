package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime settings loaded from the environment.
type Config struct {
	DBPath                string
	MasterKey             string
	AdminAPIKey           string
	TrustForwardedHeaders bool
	Port                  string

	ForwardTimeout    time.Duration
	ForwardMaxRetries int
	ForwardRetryBase  time.Duration

	RateLimitMax    int
	RateLimitWindow time.Duration

	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// Load reads configuration from environment variables and validates required fields.
func Load() (Config, error) {
	c := Config{
		DBPath:            envOr("DB_PATH", "./rules.db"),
		Port:              envOr("PORT", "8080"),
		ForwardTimeout:    durationOr("FORWARD_TIMEOUT", 30*time.Second),
		ForwardMaxRetries: intOr("FORWARD_MAX_RETRIES", 3),
		ForwardRetryBase:  durationOr("FORWARD_RETRY_BASE", 200*time.Millisecond),
		RateLimitMax:      intOr("RATE_LIMIT_MAX", 100),
		RateLimitWindow:   durationOr("RATE_LIMIT_WINDOW", time.Minute),
		ReadHeaderTimeout: durationOr("READ_HEADER_TIMEOUT", 10*time.Second),
		ReadTimeout:       durationOr("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:      durationOr("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:       durationOr("IDLE_TIMEOUT", 120*time.Second),
	}

	c.MasterKey = os.Getenv("MASTER_KEY")
	if c.MasterKey == "" || len(c.MasterKey) != 64 {
		return Config{}, fmt.Errorf("MASTER_KEY is required and must be 64 hexadecimal characters")
	}

	c.AdminAPIKey = strings.TrimSpace(os.Getenv("ADMIN_API_KEY"))
	if c.AdminAPIKey == "" {
		return Config{}, fmt.Errorf("ADMIN_API_KEY is required")
	}

	c.TrustForwardedHeaders = strings.TrimSpace(os.Getenv("TRUST_FORWARDED_HEADERS")) == "1" ||
		strings.EqualFold(os.Getenv("TRUST_FORWARDED_HEADERS"), "true")

	return c, nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func durationOr(key string, def time.Duration) time.Duration {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

func intOr(key string, def int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
