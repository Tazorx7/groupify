package topics

import "time"

// Topic is the internal representation of a topic row.
type Topic struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Capacity       int       `json:"capacity"`
	CurrentMembers int       `json:"current_members"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Availability status values shown to students.
const (
	StatusAvailable   = "available"
	StatusFull        = "full"
	StatusUnavailable = "unavailable"
)

// PublicTopic is the ONLY shape of topic data a student-facing endpoint may
// return. It is projected explicitly from SQL: there is no field that could
// carry a member name, index number or registration id.
type PublicTopic struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Capacity        int    `json:"capacity"`
	RegisteredCount int    `json:"registered_count"`
	RemainingSpaces int    `json:"remaining_spaces"`
	Status          string `json:"status"`
}

// RemainingSpaces is a convenience accessor with a floor of zero.
func (t Topic) RemainingSpaces() int {
	if t.Capacity <= t.CurrentMembers {
		return 0
	}
	return t.Capacity - t.CurrentMembers
}

// Status derives the student-facing availability label.
func (t Topic) Status() string {
	switch {
	case !t.IsActive:
		return StatusUnavailable
	case t.CurrentMembers >= t.Capacity:
		return StatusFull
	default:
		return StatusAvailable
	}
}

// ToPublic converts an internal Topic to its public projection.
func (t Topic) ToPublic() PublicTopic {
	return PublicTopic{
		ID:              t.ID,
		Name:            t.Name,
		Description:     t.Description,
		Capacity:        t.Capacity,
		RegisteredCount: t.CurrentMembers,
		RemainingSpaces: t.RemainingSpaces(),
		Status:          t.Status(),
	}
}

// AdminTopic is the management-facing shape, including lifecycle fields and
// activation state. It still contains no student data.
type AdminTopic struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Capacity        int       `json:"capacity"`
	RegisteredCount int       `json:"registered_count"`
	RemainingSpaces int       `json:"remaining_spaces"`
	IsActive        bool      `json:"is_active"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (t Topic) ToAdmin() AdminTopic {
	return AdminTopic{
		ID:              t.ID,
		Name:            t.Name,
		Description:     t.Description,
		Capacity:        t.Capacity,
		RegisteredCount: t.CurrentMembers,
		RemainingSpaces: t.RemainingSpaces(),
		IsActive:        t.IsActive,
		Status:          t.Status(),
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

// CreateRequest is the admin payload for a new topic.
type CreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Capacity    int    `json:"capacity"`
}

// UpdateRequest is a partial update. Omitted fields are left untouched, which
// is why pointers are used instead of zero values.
type UpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Capacity    *int    `json:"capacity"`
}
