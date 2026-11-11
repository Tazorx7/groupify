package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/groupify/groupify/backend/internal/auth"
	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/internal/httpx"
)

type sessionKey struct{}

// SessionCookie configures the admin session cookie.
type SessionCookie struct {
	Name   string
	TTL    time.Duration
	Secure bool
	Domain string
}

// Auth authenticates admin requests from an opaque session cookie.
type Auth struct {
	sessions *auth.Store
	cookie   SessionCookie
}

func NewAuth(sessions *auth.Store, cookie SessionCookie) *Auth {
	return &Auth{sessions: sessions, cookie: cookie}
}

// CookieName exposes the configured cookie name (used by the login handler).
func (a *Auth) CookieName() string { return a.cookie.Name }

// SetSessionCookie writes the session cookie after a successful login.
func (a *Auth) SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookie.Name,
		Value:    token,
		Path:     "/",
		Domain:   a.cookie.Domain,
		MaxAge:   int(a.cookie.TTL.Seconds()),
		HttpOnly: true,
		Secure:   a.cookie.Secure,
		// Lax: the cookie is sent on top-level navigation (so a bookmark to
		// /admin works) but not on cross-site POSTs, which blocks the simplest
		// CSRF vector. The CSRF token is the second layer.
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie expires the cookie.
func (a *Auth) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookie.Name,
		Value:    "",
		Path:     "/",
		Domain:   a.cookie.Domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.cookie.Secure,
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
			a.ClearSessionCookie(w)
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
