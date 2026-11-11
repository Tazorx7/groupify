// Package config loads and validates process configuration from the
// environment. Missing or malformed values fail fast at startup rather than
// surfacing as confusing runtime behaviour.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env               string
	HTTPAddr          string
	DatabaseURL       string
	AutoMigrate       bool
	CORSOrigins       []string
	TrustProxyHeaders bool

	SessionTTL   time.Duration
	CookieSecure bool
	CookieDomain string

	RateLimitLoginPerMinute    int
	RateLimitRegisterPerMinute int
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

// Load reads configuration. A .env file in the working directory (or up to two
// parents) is loaded first; real environment variables always win.
func Load() (*Config, error) {
	loadDotEnv()

	cfg := &Config{
		Env:                        env("APP_ENV", "dev"),
		HTTPAddr:                   env("HTTP_ADDR", ":8080"),
		DatabaseURL:                env("DATABASE_URL", ""),
		CORSOrigins:                splitList(env("CORS_ORIGINS", "")),
		TrustProxyHeaders:          boolEnv("TRUST_PROXY_HEADERS", false),
		CookieSecure:               boolEnv("COOKIE_SECURE", false),
		CookieDomain:               env("COOKIE_DOMAIN", ""),
		RateLimitLoginPerMinute:    intEnv("RATE_LIMIT_LOGIN_PER_MINUTE", 5),
		RateLimitRegisterPerMinute: intEnv("RATE_LIMIT_REGISTER_PER_MINUTE", 60),
	}

	var err error
	if cfg.AutoMigrate, err = boolEnvErr("AUTO_MIGRATE", true); err != nil {
		return nil, err
	}
	if cfg.SessionTTL, err = durationEnv("SESSION_TTL", 12*time.Hour); err != nil {
		return nil, err
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RateLimitLoginPerMinute <= 0 {
		cfg.RateLimitLoginPerMinute = 5
	}
	if cfg.RateLimitRegisterPerMinute <= 0 {
		cfg.RateLimitRegisterPerMinute = 60
	}
	if cfg.IsProduction() {
		cfg.CookieSecure = true
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	v, err := boolEnvErr(key, fallback)
	if err != nil {
		return fallback
	}
	return v
}

func boolEnvErr(key string, fallback bool) (bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return v, nil
}

func intEnv(key string, fallback int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 12h: %w", key, err)
	}
	return v, nil
}

func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv looks for .env in the working directory and up to two parents so
// `go run ./cmd/api` from backend/ still finds the repository-level file.
func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for depth := 0; depth < 3; depth++ {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			parseDotEnv(path)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func parseDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}
