// Package httpx contains transport-level helpers: JSON encoding, decoding with
// strict limits, and the single place where business errors become HTTP
// responses. Handlers never write status codes for errors themselves.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/groupify/groupify/backend/internal/domain"
)

// MaxBodyBytes caps request bodies. Registration payloads are tiny; a large cap
// only helps an attacker.
const MaxBodyBytes = 64 * 1024

type errorEnvelope struct {
	Error *domain.Error `json:"error"`
}

// JSON writes v as JSON with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status line is already sent; nothing useful left to do.
		slog.Error("write json response", "error", err)
	}
}

// NoContent writes 204.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Fail maps an error to a JSON response. Known business errors keep their code
// and message; anything else becomes a generic 500 and is logged with the
// request id (never the payload, which may contain personal data).
func Fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}

	if e := domain.AsError(err); e != nil {
		JSON(w, e.Status(), errorEnvelope{Error: e})
		return
	}

	slog.Error("unhandled request error",
		"error", err,
		"method", r.Method,
		"path", r.URL.Path,
		"request_id", middleware.GetReqID(r.Context()),
	)
	JSON(w, http.StatusInternalServerError, errorEnvelope{Error: domain.ErrInternal})
}

// DecodeJSON strictly decodes a request body into dst: unknown fields, trailing
// data and oversized bodies are rejected. Returns a *domain.Error ready to be
// passed to Fail.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) *domain.Error {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		if mediaType := strings.TrimSpace(strings.Split(ct, ";")[0]); mediaType != "application/json" {
			return domain.ErrInvalidJSON
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			return domain.ErrPayloadTooLarge
		case errors.Is(err, io.EOF):
			return domain.ErrInvalidJSON.WithField("body", "Request body is required.")
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return domain.ErrInvalidJSON
		default:
			return domain.ErrInvalidJSON.WithField("body", "Request body could not be parsed.")
		}
	}
	if dec.More() {
		return domain.ErrInvalidJSON.WithField("body", "Request body must contain a single JSON object.")
	}
	return nil
}

// QueryInt reads a non-negative integer query parameter.
func QueryInt(r *http.Request, name string, fallback int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return fallback
	}
	return v
}

// Page reads limit/offset query parameters.
func Page(r *http.Request) domain.Page {
	return domain.Page{
		Limit:  QueryInt(r, "limit", domain.DefaultPageSize),
		Offset: QueryInt(r, "offset", 0),
	}.Normalize()
}

// QueryString trims a query parameter.
func QueryString(r *http.Request, name string) string {
	return strings.TrimSpace(r.URL.Query().Get(name))
}

// RequestID returns the chi request id, if any.
func RequestID(r *http.Request) string {
	return middleware.GetReqID(r.Context())
}

// PathID parses a positive integer path parameter.
func PathID(r *http.Request, name string) (int64, *domain.Error) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.ErrNotFound.WithField(name, "Invalid identifier.")
	}
	return id, nil
}
