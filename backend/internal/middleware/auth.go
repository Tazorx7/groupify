package middleware

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/groupify/groupify/backend/internal/auth"
	"github.com/groupify/groupify/backend/internal/config"
	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/internal/httpx"
)

type sessionKey struct{}

// SessionCookie configures the admin session cookie.
//
// The Secure flag is decided per request rather than once at startup: the same
// binary is reached over HTTPS behind a proxy in production and over plain HTTP
// on localhost, and a Secure cookie is never sent over HTTP at all.
type SessionCookie struct {
	Name   string
	TTL    time.Duration
	Domain string

	// Mode comes from COOKIE_SECURE; TrustProxyHeader mirrors
	// TRUST_PROXY_HEADERS; Production mirrors APP_ENV=production and gates the
	// refusal to issue a session over plain HTTP.
	Mode       config.CookieMode
	TrustProxy bool
	Production bool
}

// Secure reports whether the cookie issued for this request carries the flag.
func (c SessionCookie) Secure(r *http.Request) bool {
	switch c.Mode {
	case config.CookieSecureAlways:
		return true
	case config.CookieSecureNever:
		return false
	default:
		return IsHTTPS(r, c.TrustProxy)
	}
}

// IsHTTPS reports whether the browser reached this process over TLS.
//
// Behind a TLS-terminating proxy the connection to us is plain HTTP, so
// X-Forwarded-Proto is the only evidence — and it is trusted only when
// explicitly configured, because any client can send that header.
func IsHTTPS(r *http.Request, trustProxy bool) bool {
	if r.TLS != nil {
		return true
	}
	if !trustProxy {
		return false
	}
	proto, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}

// IsLoopback reports whether a Host header names the local machine.
func IsLoopback(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")

	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Auth authenticates admin requests from an opaque session cookie.
type Auth struct {
	sessions *auth.Store
	cookie   SessionCookie
	warnOnce sync.Once
}

func NewAuth(sessions *auth.Store, cookie SessionCookie) *Auth {
	return &Auth{sessions: sessions, cookie: cookie}
}

// CookieName exposes the configured cookie name (used by the login handler).
func (a *Auth) CookieName() string { return a.cookie.Name }

// SetSessionCookie writes the session cookie after a successful login.
//
// It refuses to do so when the token would travel in clear text: in production,
// a session is only ever issued over HTTPS or to a loopback host.
func (a *Auth) SetSessionCookie(w http.ResponseWriter, r *http.Request, token string) error {
	secure := a.cookie.Secure(r)

	if !secure && a.cookie.Production && !IsLoopback(r.Host) {
		return domain.ErrInsecureCookie
	}
	if !secure && a.cookie.Production {
		a.warnOnce.Do(func() {
			slog.Warn("issuing an admin session cookie without the Secure flag",
				"reason", "plain HTTP request to a loopback host",
				"hint", "expected while checking the production image on localhost; reach the real deployment over HTTPS")
		})
	}

	http.SetCookie(w, &http.Cookie{
		Name:     a.cookie.Name,
		Value:    token,
		Path:     "/",
		Domain:   a.cookie.Domain,
		MaxAge:   int(a.cookie.TTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		// Lax: the cookie is sent on top-level navigation (so a bookmark to
		// /admin works) but not on cross-site POSTs, which blocks the simplest
		// CSRF vector. The CSRF token is the second layer.
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// ClearSessionCookie expires the cookie. The Secure flag has to match what was
// issued, otherwise a browser on a plain-HTTP connection ignores the deletion.
func (a *Auth) ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookie.Name,
		Value:    "",
		Path:     "/",
		Domain:   a.cookie.Domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.cookie.Secure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// Token reads the raw session token from the request cookie.
func (a *Auth) Token(r *http.Request) string {
	c, err := r.Cookie(a.cookie.Name)
	if err != nil {
		return ""
	}
	return c.Value
}

// RequireAdmin rejects unauthenticated requests with 401 and puts the session
// in the request context for downstream handlers.
func (a *Auth) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := a.Token(r)
		if token == "" {
			httpx.Fail(w, r, domain.ErrAuthenticationRequired)
			return
		}

		sess, err := a.sessions.Lookup(r.Context(), token)
		if err != nil {
			httpx.Fail(w, r, err)
			return
		}
		if sess == nil {
			a.ClearSessionCookie(w, r)
			httpx.Fail(w, r, domain.ErrAuthenticationRequired)
			return
		}

		// Sliding expiration; errors here must not fail the request.
		_ = a.sessions.Touch(r.Context(), token, a.cookie.TTL)

		ctx := context.WithValue(r.Context(), sessionKey{}, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireCSRF enforces the double-submit CSRF token on state-changing admin
// requests. It must run after RequireAdmin.
func (a *Auth) RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := SessionFrom(r.Context())
		if sess == nil {
			httpx.Fail(w, r, domain.ErrAuthenticationRequired)
			return
		}

		header := r.Header.Get("X-CSRF-Token")
		if header == "" || subtle.ConstantTimeCompare([]byte(header), []byte(sess.CSRF)) != 1 {
			httpx.Fail(w, r, domain.ErrCSRF)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SessionFrom returns the authenticated session, or nil.
func SessionFrom(ctx context.Context) *auth.Session {
	s, _ := ctx.Value(sessionKey{}).(*auth.Session)
	return s
}
