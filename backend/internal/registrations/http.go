package registrations

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/internal/httpx"
	"github.com/groupify/groupify/backend/internal/middleware"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Create handles POST /api/registrations (public, rate limited).
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if e := httpx.DecodeJSON(w, r, &req); e != nil {
		httpx.Fail(w, r, e)
		return
	}

	res, err := h.svc.Register(r.Context(), req, r.Header.Get("Idempotency-Key"), httpx.RequestID(r))
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}

	if res.Replayed {
		// Make the replay observable for debugging without changing the body.
		w.Header().Set("Idempotent-Replayed", "true")
	}
	httpx.JSON(w, res.Status, res.Confirmation)
}

// List handles GET /api/admin/registrations.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context(), filterFromRequest(r))
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// Delete handles DELETE /api/admin/registrations/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.Delete(r.Context(), sess.AdminID, id, httpx.RequestID(r)); err != nil {
		httpx.Fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// Export handles GET /api/admin/registrations/export.
//
// The CSV is streamed with a UTF-8 BOM so Excel detects the encoding; every
// field is quoted by encoding/csv, which neutralises formula-injection payloads
// that start with =, +, - or @ when the file is opened in a spreadsheet.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ExportRows(r.Context(), filterFromRequest(r))
	if err != nil {
		httpx.Fail(w, r, err)
		return
	}

	filename := fmt.Sprintf("groupify-registrations-%s.csv", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte("\xEF\xBB\xBF"))

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"index_number", "full_name", "topic", "registered_at"})
	for _, reg := range rows {
		_ = cw.Write([]string{
			sanitiseCSV(reg.IndexNumber),
			sanitiseCSV(reg.FullName),
			sanitiseCSV(reg.TopicName),
			reg.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		// Headers are already sent; the truncated file is the only signal.
		return
	}
}

func filterFromRequest(r *http.Request) Filter {
	topicID, _ := strconv.ParseInt(httpx.QueryString(r, "topic_id"), 10, 64)
	return Filter{
		Search:  httpx.QueryString(r, "search"),
		TopicID: topicID,
		SortBy:  httpx.QueryString(r, "sort"),
		SortDir: httpx.QueryString(r, "dir"),
		Page:    httpx.Page(r),
	}
}

// sanitiseCSV defuses spreadsheet formula injection by prefixing a single quote
// when a value begins with a character a spreadsheet would interpret.
func sanitiseCSV(v string) string {
	if v == "" {
		return v
	}
	switch v[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + v
	}
	return v
}
