package topics

import (
	"net/http"

	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/internal/httpx"
	"github.com/groupify/groupify/backend/internal/middleware"
)

// Handler exposes topics over HTTP. Admin routes are mounted behind the admin
// auth + CSRF middleware by the router; the handler itself never decides who
// may call it.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type publicListResponse struct {
	Topics []PublicTopic `json:"topics"`
}

type adminListResponse struct {
	Topics []AdminTopic `json:"topics"`
}

// ListPublic handles GET /api/topics.
func (h *Handler) ListPublic(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListPublic(r.Context())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, publicListResponse{Topics: list})
}

// ListAdmin handles GET /api/admin/topics.
func (h *Handler) ListAdmin(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListAdmin(r.Context())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, adminListResponse{Topics: list})
}

// Create handles POST /api/admin/topics.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFrom(r.Context())
	if sess == nil {
		httpx.Fail(w, r, domain.ErrAuthenticationRequired)
		return
	}

	var req CreateRequest
	if e := httpx.DecodeJSON(w, r, &req); e != nil {
		httpx.Fail(w, r, e)
		return
	}

	topic, err := h.svc.Create(r.Context(), sess.AdminID, req, httpx.RequestID(r))
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, topic)
}

// Update handles PATCH /api/admin/topics/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFrom(r.Context())
	if sess == nil {
		httpx.Fail(w, r, domain.ErrAuthenticationRequired)
		return
	}
	id, e := httpx.PathID(r, "id")
	if e != nil {
		httpx.Fail(w, r, e)
		return
	}

	var req UpdateRequest
	if e := httpx.DecodeJSON(w, r, &req); e != nil {
		httpx.Fail(w, r, e)
		return
	}

	topic, err := h.svc.Update(r.Context(), sess.AdminID, id, req, httpx.RequestID(r))
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, topic)
}

// Activate handles POST /api/admin/topics/{id}/activate.
func (h *Handler) Activate(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, true)
}

// Deactivate handles POST /api/admin/topics/{id}/deactivate.
func (h *Handler) Deactivate(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, false)
}

func (h *Handler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	sess := middleware.SessionFrom(r.Context())
	if sess == nil {
		httpx.Fail(w, r, domain.ErrAuthenticationRequired)
		return
	}
	id, e := httpx.PathID(r, "id")
	if e != nil {
		httpx.Fail(w, r, e)
		return
	}

	topic, err := h.svc.SetActive(r.Context(), sess.AdminID, id, active, httpx.RequestID(r))
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, topic)
}
