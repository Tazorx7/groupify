package config

import (
	"strings"
	"testing"
)

// baseEnv satisfies Load's only hard requirement and keeps the developer's
// .env from leaking into the assertions (real environment variables win over
// the file, and every key these tests care about is set explicitly).
func baseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable")
	t.Setenv("APP_ENV", "dev")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("TRUST_PROXY_HEADERS", "false")
}

func TestCookieModeDefaultsToAuto(t *testing.T) {
	baseEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CookieMode != CookieSecureAuto {
		t.Fatalf("CookieMode = %q, want %q", cfg.CookieMode, CookieSecureAuto)
	}
}

func TestCookieModeParsing(t *testing.T) {
	cases := map[string]CookieMode{
		"":       CookieSecureAuto,
		"auto":   CookieSecureAuto,
		" AUTO ": CookieSecureAuto,
		"always": CookieSecureAlways,
		"true":   CookieSecureAlways,
		"1":      CookieSecureAlways,
		"never":  CookieSecureNever,
		"false":  CookieSecureNever,
		"0":      CookieSecureNever,
		"FALSE":  CookieSecureNever,
		"yes":    CookieSecureAlways,
		"no":     CookieSecureNever,
	}

	for raw, want := range cases {
		baseEnv(t)
		t.Setenv("COOKIE_SECURE", raw)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("COOKIE_SECURE=%q: %v", raw, err)
		}
		if cfg.CookieMode != want {
			t.Fatalf("COOKIE_SECURE=%q: mode = %q, want %q", raw, cfg.CookieMode, want)
		}
	}
}

func TestCookieModeRejectsNonsense(t *testing.T) {
	baseEnv(t)
	t.Setenv("COOKIE_SECURE", "sometimes")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for an unrecognised COOKIE_SECURE")
	}
	if !strings.Contains(err.Error(), "COOKIE_SECURE") {
		t.Fatalf("error does not name the variable: %v", err)
	}
}

// Production must not be able to run with the Secure flag disabled: the admin
// session would cross the wire in clear text. Failing at startup is the only
// honest behaviour.
func TestProductionRefusesInsecureCookies(t *testing.T) {
	baseEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "false")

	cfg, err := Load()
	if err == nil {
		t.Fatalf("expected failure, got mode %q", cfg.CookieMode)
	}
	if !strings.Contains(err.Error(), "APP_ENV=production") {
		t.Fatalf("error should explain the combination: %v", err)
	}
}

func TestProductionWarnsWhenProxyTrustIsOff(t *testing.T) {
	baseEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "auto")
	t.Setenv("TRUST_PROXY_HEADERS", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one about proxy trust", cfg.Warnings)
	}
	if !strings.Contains(strings.Join(cfg.Warnings, " "), "TRUST_PROXY_HEADERS") {
		t.Fatalf("warning does not mention the fix: %v", cfg.Warnings)
	}
}

func TestProductionIsQuietWhenProxyTrustIsOn(t *testing.T) {
	baseEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "auto")
	t.Setenv("TRUST_PROXY_HEADERS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", cfg.Warnings)
	}
}

// Development keeps working over plain HTTP without any ceremony.
func TestDevelopmentAllowsInsecureCookies(t *testing.T) {
	baseEnv(t)
	t.Setenv("APP_ENV", "dev")
	t.Setenv("COOKIE_SECURE", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CookieMode != CookieSecureNever {
		t.Fatalf("CookieMode = %q, want never", cfg.CookieMode)
	}
	if len(cfg.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", cfg.Warnings)
	}
}

func TestParseDotEnvLine(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		wantKey   string
		wantValue string
		wantOK    bool
	}{
		{name: "plain", line: "APP_ENV=dev", wantKey: "APP_ENV", wantValue: "dev", wantOK: true},
		{name: "spaces around", line: "  APP_ENV = dev  ", wantKey: "APP_ENV", wantValue: "dev", wantOK: true},
		{name: "trailing comment", line: "COOKIE_SECURE=false   # only in production", wantKey: "COOKIE_SECURE", wantValue: "false", wantOK: true},
		{name: "comment on its own", line: "# COOKIE_SECURE=false", wantOK: false},
		{name: "blank", line: "   ", wantOK: false},
		{name: "no separator", line: "JUSTAWORD", wantOK: false},
		{name: "export prefix", line: "export APP_ENV=production", wantKey: "APP_ENV", wantValue: "production", wantOK: true},
		{name: "double quoted keeps hash", line: `PASSWORD="p#ss=word"`, wantKey: "PASSWORD", wantValue: "p#ss=word", wantOK: true},
		{name: "single quoted keeps hash", line: `PASSWORD='p#ss word'`, wantKey: "PASSWORD", wantValue: "p#ss word", wantOK: true},
		{name: "quoted then comment", line: `NAME="a b"  # note`, wantKey: "NAME", wantValue: "a b", wantOK: true},
		{name: "hash inside a value", line: "PASSWORD=abc#def", wantKey: "PASSWORD", wantValue: "abc#def", wantOK: true},
		{name: "empty value", line: "STATIC_DIR=", wantKey: "STATIC_DIR", wantValue: "", wantOK: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, value, ok := parseDotEnvLine(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if key != tc.wantKey || value != tc.wantValue {
				t.Fatalf("got %q=%q, want %q=%q", key, value, tc.wantKey, tc.wantValue)
			}
		})
	}
}
