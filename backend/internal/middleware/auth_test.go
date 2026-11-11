package middleware

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/groupify/groupify/backend/internal/config"
	"github.com/groupify/groupify/backend/internal/domain"
)

func TestIsHTTPS(t *testing.T) {
	cases := []struct {
		name       string
		tls        bool
		trustProxy bool
		header     string
		want       bool
	}{
		{name: "direct TLS", tls: true, want: true},
		{name: "plain http", want: false},
		{name: "proxy header trusted", trustProxy: true, header: "https", want: true},
		{name: "proxy header ignored without trust", header: "https", want: false},
		{name: "proxy header http", trustProxy: true, header: "http", want: false},
		// A comma list is what several proxies emit; the first value is the
		// original client's scheme.
		{name: "comma list takes the first", trustProxy: true, header: "https, http", want: true},
		{name: "comma list http first", trustProxy: true, header: "http, https", want: false},
		{name: "case insensitive", trustProxy: true, header: "HTTPS", want: true},
		{name: "TLS wins over a lying header", tls: true, trustProxy: true, header: "http", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if tc.header != "" {
				r.Header.Set("X-Forwarded-Proto", tc.header)
			}
			if got := IsHTTPS(r, tc.trustProxy); got != tc.want {
				t.Fatalf("IsHTTPS = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsLoopback(t *testing.T) {
	loopback := []string{"localhost", "localhost:8080", "127.0.0.1", "127.0.0.1:8080", "[::1]:8080", "::1", "app.localhost:8080"}
	for _, host := range loopback {
		if !IsLoopback(host) {
			t.Fatalf("IsLoopback(%q) = false, want true", host)
		}
	}

	notLoopback := []string{"example.com", "example.com:443", "192.168.1.5:8080", "10.0.0.2", "0.0.0.0:8080", ""}
	for _, host := range notLoopback {
		if IsLoopback(host) {
			t.Fatalf("IsLoopback(%q) = true, want false", host)
		}
	}
}

func cookieSecure(t *testing.T, mode config.CookieMode, req *http.Request) bool {
	t.Helper()
	rec := httptest.NewRecorder()
	auth := NewAuth(nil, SessionCookie{
		Name: "session",
		TTL:  time.Hour,
		Mode: mode,
	})
	if err := auth.SetSessionCookie(rec, req, "token"); err != nil {
		t.Fatalf("SetSessionCookie: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v, want exactly one", cookies)
	}
	return cookies[0].Secure
}

// The flag follows the request, which is the whole point: one binary, HTTPS in
// production and plain HTTP on localhost.
func TestSecureFlagFollowsTheRequest(t *testing.T) {
	cases := []struct {
		name string
		mode config.CookieMode
		tls  bool
		want bool
	}{
		{name: "auto over TLS", mode: config.CookieSecureAuto, tls: true, want: true},
		{name: "auto over http", mode: config.CookieSecureAuto, want: false},
		{name: "always over http", mode: config.CookieSecureAlways, want: true},
		{name: "never over TLS", mode: config.CookieSecureNever, tls: true, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
			r.Host = "127.0.0.1:8080"
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if got := cookieSecure(t, tc.mode, r); got != tc.want {
				t.Fatalf("Secure = %v, want %v", got, tc.want)
			}
		})
	}
}

func productionAuth(mode config.CookieMode) *Auth {
	return NewAuth(nil, SessionCookie{
		Name:       "session",
		TTL:        time.Hour,
		Mode:       mode,
		Production: true,
	})
}

// The failure this replaces: a production image checked over http://<lan-ip>
// used to hand out a cookie the browser then refused to send, so sign-in
// appeared to work and the session silently vanished. Now it is refused with a
// message that says what to do.
func TestProductionRefusesSessionOverPlainHTTP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	r.Host = "192.168.1.5:8080"

	rec := httptest.NewRecorder()
	err := productionAuth(config.CookieSecureAuto).SetSessionCookie(rec, r, "token")

	if !errors.Is(err, domain.ErrInsecureCookie) {
		t.Fatalf("err = %v, want ErrInsecureCookie", err)
	}
	if got := rec.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("a cookie was written anyway: %q", got)
	}
}

// ...but the same production build is usable on the machine itself, which is
// how a deploy gets checked before it is trusted.
func TestProductionAllowsSessionOnLoopback(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	r.Host = "127.0.0.1:8080"

	rec := httptest.NewRecorder()
	if err := productionAuth(config.CookieSecureAuto).SetSessionCookie(rec, r, "token"); err != nil {
		t.Fatalf("SetSessionCookie: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v, want one", cookies)
	}
	if cookies[0].Secure {
		t.Fatal("loopback cookie should not claim Secure: the browser would drop it on http")
	}
}

func TestProductionAllowsSessionOverTLS(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	r.Host = "groupify.example.edu"
	r.TLS = &tls.ConnectionState{}

	rec := httptest.NewRecorder()
	if err := productionAuth(config.CookieSecureAuto).SetSessionCookie(rec, r, "token"); err != nil {
		t.Fatalf("SetSessionCookie: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure {
		t.Fatalf("cookies = %v, want one Secure cookie", cookies)
	}
}

// Behind a TLS-terminating proxy the process sees plain HTTP; the trusted
// header is what makes the cookie Secure.
func TestProductionTrustsProxyProtoHeaderWhenConfigured(t *testing.T) {
	auth := NewAuth(nil, SessionCookie{
		Name:       "session",
		TTL:        time.Hour,
		Mode:       config.CookieSecureAuto,
		Production: true,
		TrustProxy: true,
	})

	r := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	r.Host = "groupify.example.edu"
	r.Header.Set("X-Forwarded-Proto", "https")

	rec := httptest.NewRecorder()
	if err := auth.SetSessionCookie(rec, r, "token"); err != nil {
		t.Fatalf("SetSessionCookie: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure {
		t.Fatalf("cookies = %v, want one Secure cookie", cookies)
	}
}

// Deletion has to carry the same flag as issuance, or a browser on plain HTTP
// ignores it and the user stays "signed in" to a session that is gone.
func TestClearUsesTheSameSecureFlag(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil)
	r.Host = "127.0.0.1:8080"

	rec := httptest.NewRecorder()
	productionAuth(config.CookieSecureAuto).ClearSessionCookie(rec, r)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v, want one", cookies)
	}
	if cookies[0].Secure || cookies[0].MaxAge >= 0 {
		t.Fatalf("cookie = %+v, want an insecure, expired cookie", cookies[0])
	}
}
