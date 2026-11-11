package audit

import (
	"net/http"

	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/internal/httpx"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler { return &Handler{store: store} }

// List handles GET /api/admin/audit-logs.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page := httpx.Page(r)
	entries, total, err := h.store.List(r.Context(), Query{
		Action:     httpx.QueryString(r, "action"),
		EntityType: httpx.QueryString(r, "entity_type"),
		Limit:      page.Limit,
		Offset:     page.Offset,
	})
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, domain.NewListResult(entries, total, page))
}
