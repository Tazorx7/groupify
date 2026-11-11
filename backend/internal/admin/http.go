package admin

import (
	"net/http"

	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/internal/httpx"
	"github.com/groupify/groupify/backend/internal/middleware"
)

// SessionCookieWriter is the subset of the auth middleware the handler needs.
type SessionCookieWriter interface {
	SetSessionCookie(w http.ResponseWriter, token string)
	ClearSessionCookie(w http.ResponseWriter)
	Token(r *http.Request) string
}

type Handler struct {
	svc    *Service
	cookie SessionCookieWriter
}

func NewHandler(svc *Service, cookie SessionCookieWriter) *Handler {
	return &Handler{svc: svc, cookie: cookie}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Admin     Account `json:"admin"`
	CSRFToken string  `json:"csrf_token"`
	ExpiresAt string  `json:"expires_at"`
}

// Login handles POST /api/admin/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if e := httpx.DecodeJSON(w, r, &req); e != nil {
		httpx.Fail(w, r, e)
		return
	}

	res, err := h.svc.Authenticate(r.Context(), req.Email, req.Password, httpx.RequestID(r))
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}

	h.cookie.SetSessionCookie(w, res.Token)
	httpx.JSON(w, http.StatusOK, loginResponse{
		Admin:     res.Account,
		CSRFToken: res.CSRFToken,
		ExpiresAt: res.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Logout handles POST /api/admin/logout. The session is revoked server-side, so
// a copied cookie stops working immediately.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFrom(r.Context())
	if sess == nil {
		httpx.Fail(w, r, domain.ErrAuthenticationRequired)
		return
	}
	if err := h.svc.Logout(r.Context(), h.cookie.Token(r), sess.AdminID, httpx.RequestID(r)); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	h.cookie.ClearSessionCookie(w)
	httpx.NoContent(w)
}

// CurrentSession handles GET /api/admin/session. The SPA calls it on load to
// recover the CSRF token after a page refresh (the token is not stored in a
// script-readable cookie).
func (h *Handler) CurrentSession(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFrom(r.Context())
	if sess == nil {
		httpx.Fail(w, r, domain.ErrAuthenticationRequired)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"admin":      Account{ID: sess.AdminID, Email: sess.Email},
		"csrf_token": sess.CSRF,
		"expires_at": sess.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Overview handles GET /api/admin/overview.
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	o, err := h.svc.GetOverview(r.Context())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, o)
}
