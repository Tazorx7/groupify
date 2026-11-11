package registrations

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/groupify/groupify/backend/internal/domain"
)

// Registration is the internal row representation. It is never serialised
// directly to a student; handlers choose between Confirmation (student) and
// AdminRegistration (admin).
type Registration struct {
	ID          int64
	FullName    string
	IndexNumber string
	TopicID     int64
	TopicName   string
	CreatedAt   time.Time
}

// Confirmation is what a student receives after a successful registration. It
// echoes only their own submission plus the topic they picked.
type Confirmation struct {
	RegistrationID int64     `json:"registration_id"`
	FullName       string    `json:"full_name"`
	IndexNumber    string    `json:"index_number"`
	Topic          TopicRef  `json:"topic"`
	RegisteredAt   time.Time `json:"registered_at"`
}

// TopicRef is a minimal topic reference.
type TopicRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// AdminRegistration is the management view of a registration.
type AdminRegistration struct {
	ID          int64     `json:"id"`
	FullName    string    `json:"full_name"`
	IndexNumber string    `json:"index_number"`
	TopicID     int64     `json:"topic_id"`
	TopicName   string    `json:"topic_name"`
	CreatedAt   time.Time `json:"created_at"`
}

func (r Registration) ToConfirmation() Confirmation {
	return Confirmation{
		RegistrationID: r.ID,
		FullName:       r.FullName,
		IndexNumber:    r.IndexNumber,
		Topic:          TopicRef{ID: r.TopicID, Name: r.TopicName},
		RegisteredAt:   r.CreatedAt,
	}
}

func (r Registration) ToAdmin() AdminRegistration {
	return AdminRegistration{
		ID:          r.ID,
		FullName:    r.FullName,
		IndexNumber: r.IndexNumber,
		TopicID:     r.TopicID,
		TopicName:   r.TopicName,
		CreatedAt:   r.CreatedAt,
	}
}

// CreateRequest is the student submission payload.
type CreateRequest struct {
	FullName    string `json:"full_name"`
	IndexNumber string `json:"index_number"`
	TopicID     int64  `json:"topic_id"`
}

// Result carries the confirmation plus whether it was replayed from an
// idempotency record (in which case the original status code is reused).
type Result struct {
	Confirmation *Confirmation
	Status       int
	Replayed     bool
}

// Filter drives admin listing and CSV export.
type Filter struct {
	Search  string
	TopicID int64
	SortBy  string
	SortDir string
	Page    domain.Page
}

// Normalize applies defaults and whitelists sort fields. The sort column is
// mapped to a fixed SQL fragment, never interpolated from user input.
func (f Filter) Normalize() Filter {
	switch f.SortBy {
	case "full_name", "index_number", "topic", "registered_at":
	default:
		f.SortBy = "registered_at"
	}
	switch f.SortDir {
	case "asc", "desc":
	default:
		f.SortDir = "desc"
	}
	f.Page = f.Page.Normalize()
	return f
}

// requestHash fingerprints a submission so an idempotency key reused with a
// different payload can be rejected instead of silently returning the first
// response.
func requestHash(fullName, indexNumber string, topicID int64) string {
	sum := sha256.Sum256([]byte(fullName + "\x00" + indexNumber + "\x00" + strconv.FormatInt(topicID, 10)))
	return hex.EncodeToString(sum[:])
}
