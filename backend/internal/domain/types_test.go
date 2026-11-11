package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/groupify/groupify/backend/internal/domain"
)

func TestNormalizeIndexNumber(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"trims surrounding space", "  12345678  ", "12345678"},
		{"collapses internal space", "CS  2024   045", "CS 2024 045"},
		{"upper-cases", "cs/2024/045", "CS/2024/045"},
		{"keeps leading zeroes", "00123456", "00123456"},
		{"non-breaking space", "\u00a0100234\u00a0", "100234"},
		{"tabs and newlines", "\t100\n234\t", "100 234"},
		{"already normal", "A-100", "A-100"},
		{"empty", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.NormalizeIndexNumber(tc.in); got != tc.want {
				t.Fatalf("NormalizeIndexNumber(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"  Ama   Mensah ", "Ama Mensah"},
		{"Kofi\u00a0Boateng", "Kofi Boateng"},
		{"Yaa", "Yaa"},
	}
	for _, tc := range cases {
		if got := domain.NormalizeName(tc.in); got != tc.want {
			t.Fatalf("NormalizeName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := domain.NormalizeEmail("  Rep@Example.EDU "); got != "rep@example.edu" {
		t.Fatalf("got %q", got)
	}
}

func TestValidateStudentInputAcceptsValidInput(t *testing.T) {
	name, index, err := domain.ValidateStudentInput("  Ama  Mensah ", " cs/2024/045 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Ama Mensah" {
		t.Fatalf("name = %q", name)
	}
	if index != "CS/2024/045" {
		t.Fatalf("index = %q", index)
	}
}

func TestValidateStudentInputRejects(t *testing.T) {
	cases := []struct {
		name      string
		fullName  string
		index     string
		wantField string
	}{
		{"blank name", "   ", "12345678", "full_name"},
		{"name without letters", "12345", "12345678", "full_name"},
		{"blank index", "Ama Mensah", "  ", "index_number"},
		{"index with symbols", "Ama Mensah", "1234<script>", "index_number"},
		{"index too long", "Ama Mensah", strings.Repeat("9", 41), "index_number"},
		{"name too long", strings.Repeat("a", 121), "12345678", "full_name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := domain.ValidateStudentInput(tc.fullName, tc.index)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if err.Status() != 422 {
				t.Fatalf("status = %d, want 422", err.Status())
			}
			if _, ok := err.Fields[tc.wantField]; !ok {
				t.Fatalf("expected field %q in %v", tc.wantField, err.Fields)
			}
		})
	}
}

func TestValidateTopicInput(t *testing.T) {
	name, desc, err := domain.ValidateTopicInput("  Data   Structures ", "  Trees and graphs  ", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Data Structures" || desc != "Trees and graphs" {
		t.Fatalf("got name=%q desc=%q", name, desc)
	}

	for _, capacity := range []int{0, -1, 1001} {
		if _, _, err := domain.ValidateTopicInput("X", "", capacity); err == nil {
			t.Fatalf("capacity %d should be rejected", capacity)
		}
	}
}

// Errors are shared package-level sentinels; WithField must never mutate them.
func TestWithFieldDoesNotMutateSentinel(t *testing.T) {
	derived := domain.ErrTopicFull.WithField("topic_id", "gone")
	if len(domain.ErrTopicFull.Fields) != 0 {
		t.Fatal("sentinel was mutated by WithField")
	}
	if derived.Fields["topic_id"] != "gone" {
		t.Fatal("derived error lost its field")
	}
	if !errors.Is(derived, domain.ErrTopicFull) {
		t.Fatal("derived error should unwrap to the sentinel")
	}
}

func TestErrorStatusCodes(t *testing.T) {
	cases := map[*domain.Error]int{
		domain.ErrRegistrationClosed:     409,
		domain.ErrTopicFull:              409,
		domain.ErrTopicInactive:          409,
		domain.ErrTopicNotFound:          404,
		domain.ErrIndexNumberTaken:       409,
		domain.ErrInvalidCredentials:     401,
		domain.ErrAuthenticationRequired: 401,
		domain.ErrCSRF:                   403,
		domain.ErrRateLimited:            429,
		domain.ErrInternal:               500,
	}
	for e, want := range cases {
		if got := e.Status(); got != want {
			t.Fatalf("%s status = %d, want %d", e.Code, got, want)
		}
	}
}

func TestPageNormalize(t *testing.T) {
	if got := (domain.Page{}).Normalize(); got.Limit != domain.DefaultPageSize || got.Offset != 0 {
		t.Fatalf("defaults = %+v", got)
	}
	if got := (domain.Page{Limit: 5000, Offset: -3}).Normalize(); got.Limit != domain.MaxPageSize || got.Offset != 0 {
		t.Fatalf("clamp = %+v", got)
	}
}
