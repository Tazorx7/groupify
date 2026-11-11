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
	CookieMode   CookieMode
	CookieDomain string

	// Warnings are non-fatal configuration problems worth telling the operator
	// about at startup. Whoever calls Load decides how to surface them.
	Warnings []string

	RateLimitLoginPerMinute    int
	RateLimitRegisterPerMinute int

	// StaticDir, when set, points at the built SPA. The API then serves it
	// from the same origin as /api, so session cookies stay first-party and
	// CORS is unnecessary.
	StaticDir string
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

// CookieMode decides when the session cookie carries the Secure flag.
//
// The flag cannot be a single boolean decided at startup: the same binary is
// reached over HTTPS behind a proxy in production and over plain HTTP on
// localhost, and a cookie marked Secure is never sent over HTTP at all. Getting
// that wrong does not produce an error — sign-in "works", the session simply
// never comes back, which is a miserable thing to debug.
type CookieMode string

const (
	// CookieSecureAuto marks the cookie Secure exactly when the request that
	// issued it arrived over HTTPS (including via a trusted proxy header).
	// Correct behind TLS termination, correct on localhost, correct for a
	// production image someone is checking over HTTP.
	CookieSecureAuto CookieMode = "auto"
	// CookieSecureAlways is for deployments that terminate TLS somewhere the
	// process cannot see and cannot be configured to trust.
	CookieSecureAlways CookieMode = "always"
	// CookieSecureNever exists for plain-HTTP development. Production refuses
	// to start with it.
	CookieSecureNever CookieMode = "never"
)

// parseCookieMode accepts the keywords and the booleans people already have in
// their environment files.
func parseCookieMode(raw string) (CookieMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return CookieSecureAuto, nil
	case "always", "true", "1", "t", "yes":
		return CookieSecureAlways, nil
	case "never", "false", "0", "f", "no":
		return CookieSecureNever, nil
	default:
		return "", fmt.Errorf("COOKIE_SECURE must be auto, always or never (got %q)", raw)
	}
}

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
		CookieMode:                 CookieSecureAuto,
		CookieDomain:               env("COOKIE_DOMAIN", ""),
		RateLimitLoginPerMinute:    intEnv("RATE_LIMIT_LOGIN_PER_MINUTE", 5),
		RateLimitRegisterPerMinute: intEnv("RATE_LIMIT_REGISTER_PER_MINUTE", 60),
		StaticDir:                  env("STATIC_DIR", ""),
	}

	var err error
	if cfg.AutoMigrate, err = boolEnvErr("AUTO_MIGRATE", true); err != nil {
		return nil, err
	}
	if cfg.SessionTTL, err = durationEnv("SESSION_TTL", 12*time.Hour); err != nil {
		return nil, err
	}
	if cfg.CookieMode, err = parseCookieMode(env("COOKIE_SECURE", "")); err != nil {
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

	// Turning the Secure flag off in production is not a trade-off, it is a
	// broken deployment: the admin session would travel in clear text. Refuse
	// to start rather than serve it and hope nobody notices.
	if cfg.IsProduction() && cfg.CookieMode == CookieSecureNever {
		return nil, fmt.Errorf(
			"COOKIE_SECURE=false is not allowed with APP_ENV=production: " +
				"the admin session cookie would be sent over plain HTTP. " +
				"Leave it unset (auto) or set COOKIE_SECURE=always")
	}
	if cfg.IsProduction() && cfg.CookieMode == CookieSecureAuto && !cfg.TrustProxyHeaders {
		cfg.Warnings = append(cfg.Warnings,
			"APP_ENV=production with COOKIE_SECURE=auto and TRUST_PROXY_HEADERS=false: "+
				"cookies are Secure only when TLS terminates on this process. "+
				"If a proxy terminates TLS in front of it, set TRUST_PROXY_HEADERS=true "+
				"(or COOKIE_SECURE=always) or the session will not be marked Secure.")
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
		key, value, ok := parseDotEnvLine(scanner.Text())
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

// parseDotEnvLine understands the parts of the format people actually write:
// blank lines, full-line comments, `export` prefixes, quoted values, and
// trailing comments.
//
// The trailing comment case is not cosmetic. `COOKIE_SECURE=false  # only in
// production` used to load the entire string as the value, and the process then
// died with "COOKIE_SECURE must be auto, always or never" — pointing at the
// wrong problem. Values that are quoted are taken literally, so a password
// containing `#` survives.
func parseDotEnvLine(raw string) (key, value string, ok bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")

	key, value, found := strings.Cut(line, "=")
	if !found {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)

	if len(value) > 0 && (value[0] == '"' || value[0] == '\'') {
		quote := value[0]
		if end := strings.IndexByte(value[1:], quote); end >= 0 {
			return key, value[1 : 1+end], true
		}
		return key, strings.Trim(value, `"'`), true
	}

	// Unquoted: a `#` that starts the value or follows whitespace begins a
	// comment. `abc#def` is a value, which matters for generated passwords.
	if i := strings.IndexFunc(value, func(r rune) bool { return r == '#' }); i >= 0 {
		if i == 0 || value[i-1] == ' ' || value[i-1] == '\t' {
			value = value[:i]
		}
	}

	return key, strings.TrimSpace(value), true
}
