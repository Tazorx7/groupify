package settings

import (
	"net/http"

	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/internal/httpx"
	"github.com/groupify/groupify/backend/internal/middleware"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type statusResponse struct {
	RegistrationOpen bool `json:"registration_open"`
}

// PublicStatus handles GET /api/registration-status.
func (h *Handler) PublicStatus(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Get(r.Context())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, statusResponse{RegistrationOpen: st.RegistrationOpen})
}

// Get handles GET /api/admin/settings.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Get(r.Context())
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, st)
}

type updateRequest struct {
	RegistrationOpen *bool `json:"registration_open"`
}

// Update handles PATCH /api/admin/settings.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFrom(r.Context())
	if sess == nil {
		httpx.Fail(w, r, domain.ErrAuthenticationRequired)
		return
	}

	var req updateRequest
	if e := httpx.DecodeJSON(w, r, &req); e != nil {
		httpx.Fail(w, r, e)
		return
	}
	if req.RegistrationOpen == nil {
		httpx.Fail(w, r, domain.ValidationError("Please correct the highlighted fields.",
			map[string]string{"registration_open": "This field is required."}))
		return
	}

	st, err := h.svc.SetOpen(r.Context(), sess.AdminID, *req.RegistrationOpen, httpx.RequestID(r))
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, st)
}
