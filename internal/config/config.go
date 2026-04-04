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
	DevMode               bool // relaxed security for development

	ForwardTimeout    time.Duration
	ForwardMaxRetries int
	ForwardRetryBase  time.Duration

	// Rate limiting
	RateLimitMax    int
	RateLimitWindow time.Duration
	RateLimitBurst  int     // token bucket burst size
	RateLimitRefill float64 // tokens per second

	// Brute force protection
	BruteForceMaxFailures int
	BruteForceLockoutBase time.Duration
	BruteForceMaxLockout  time.Duration

	// CORS
	CORSOrigins     []string // comma-separated list or "*"
	CORSCredentials bool

	// TLS
	TLSEnabled    bool
	TLSCertFile   string
	TLSKeyFile    string
	TLSAutoGen    bool
	TLSAutoGenDir string

	// Audit logging
	AuditEnabled     bool
	AuditFilePath    string
	AuditMinSeverity string

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

		// Rate limiting defaults (token bucket)
		RateLimitMax:    intOr("RATE_LIMIT_MAX", 100),
		RateLimitWindow: durationOr("RATE_LIMIT_WINDOW", time.Minute),
		RateLimitBurst:  intOr("RATE_LIMIT_BURST", 20),
		RateLimitRefill: float64(intOr("RATE_LIMIT_REFILL_PER_SEC", 10)),

		// Brute force protection defaults
		BruteForceMaxFailures: intOr("BRUTE_FORCE_MAX_FAILURES", 5),
		BruteForceLockoutBase: durationOr("BRUTE_FORCE_LOCKOUT_BASE", 5*time.Minute),
		BruteForceMaxLockout:  durationOr("BRUTE_FORCE_MAX_LOCKOUT", time.Hour),

		// TLS defaults
		TLSAutoGenDir: "./certs",

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

	c.TrustForwardedHeaders = parseBool(os.Getenv("TRUST_FORWARDED_HEADERS"))
	c.DevMode = parseBool(os.Getenv("DEV_MODE"))

	// CORS configuration
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		c.CORSOrigins = strings.Split(v, ",")
		for i := range c.CORSOrigins {
			c.CORSOrigins[i] = strings.TrimSpace(c.CORSOrigins[i])
		}
	}
	c.CORSCredentials = parseBool(os.Getenv("CORS_CREDENTIALS"))

	// TLS configuration
	c.TLSEnabled = parseBool(os.Getenv("TLS_ENABLED"))
	c.TLSCertFile = os.Getenv("TLS_CERT_FILE")
	c.TLSKeyFile = os.Getenv("TLS_KEY_FILE")
	c.TLSAutoGen = parseBool(os.Getenv("TLS_AUTO_GENERATE"))
	if v := os.Getenv("TLS_AUTO_GENERATE_DIR"); v != "" {
		c.TLSAutoGenDir = v
	}

	// Audit logging configuration
	c.AuditEnabled = parseBool(os.Getenv("AUDIT_ENABLED"))
	c.AuditFilePath = os.Getenv("AUDIT_FILE_PATH")
	c.AuditMinSeverity = envOr("AUDIT_MIN_SEVERITY", "info")

	return c, nil
}

func parseBool(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "1" || s == "true" || s == "yes"
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
