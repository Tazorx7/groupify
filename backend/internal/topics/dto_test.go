package topics_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/groupify/groupify/backend/internal/topics"
)

// The student-facing payload is a separate type from the internal Topic. This
// test fails if anyone ever adds a member-bearing field to it, which is the
// cheapest possible guard for PRD §6 / FR-09.
func TestPublicTopicJSONHasOnlyPublicFields(t *testing.T) {
	raw, err := json.Marshal(topics.PublicTopic{
		ID:              7,
		Name:            "Algorithms",
		Description:     "Graphs and sorting",
		Capacity:        30,
		RegisteredCount: 12,
		RemainingSpaces: 18,
		Status:          topics.StatusAvailable,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	allowed := map[string]bool{
		"id": true, "name": true, "description": true, "capacity": true,
		"registered_count": true, "remaining_spaces": true, "status": true,
	}
	for key := range decoded {
		if !allowed[key] {
			t.Fatalf("unexpected public field %q in %s", key, raw)
		}
	}

	// Defence in depth: no PII-ish key may appear anywhere in the payload.
	forbidden := []string{"full_name", "index_number", "student", "member", "email", "registration_id"}
	lower := strings.ToLower(string(raw))
	for _, word := range forbidden {
		if strings.Contains(lower, word) {
			t.Fatalf("public payload contains %q: %s", word, raw)
		}
	}
}

func TestTopicStatusAndRemaining(t *testing.T) {
	cases := []struct {
		name   string
		topic  topics.Topic
		status string
		remain int
	}{
		{"available", topics.Topic{Capacity: 10, CurrentMembers: 3, IsActive: true}, topics.StatusAvailable, 7},
		{"full", topics.Topic{Capacity: 3, CurrentMembers: 3, IsActive: true}, topics.StatusFull, 0},
		{"over capacity is still zero remaining", topics.Topic{Capacity: 3, CurrentMembers: 4, IsActive: true}, topics.StatusFull, 0},
		{"inactive wins over full", topics.Topic{Capacity: 3, CurrentMembers: 3, IsActive: false}, topics.StatusUnavailable, 0},
		{"inactive", topics.Topic{Capacity: 10, CurrentMembers: 0, IsActive: false}, topics.StatusUnavailable, 10},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.topic.Status(); got != tc.status {
				t.Fatalf("status = %q, want %q", got, tc.status)
			}
			if got := tc.topic.RemainingSpaces(); got != tc.remain {
				t.Fatalf("remaining = %d, want %d", got, tc.remain)
			}
		})
	}
}
