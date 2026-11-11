package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Normalisation rules
//
// Index numbers are strings, never integers: leading zeroes are significant
// and institutional formats often contain letters and separators
// ("cs/2024/045", " 00123456 "). The canonical form is:
//
//  1. trim surrounding whitespace (including non-breaking spaces),
//  2. collapse any run of internal whitespace into a single space,
//  3. upper-case using Unicode simple case mapping.
//
// Steps 1-2 are applied to names; names additionally keep their original case
// and never have internal separators rewritten.
const (
	MaxNameRunes        = 120
	MaxIndexNumberRunes = 40
)

// CollapseSpace trims the ends and reduces internal whitespace runs to a single
// ASCII space. Non-breaking spaces are treated as whitespace.
func CollapseSpace(raw string) string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\u00a0' || r == '\u200b'
	})
	return strings.Join(fields, " ")
}

// NormalizeIndexNumber returns the canonical storage form of an index number.
func NormalizeIndexNumber(raw string) string {
	return strings.ToUpper(CollapseSpace(raw))
}

// NormalizeName returns the canonical storage form of a student name.
func NormalizeName(raw string) string {
	return CollapseSpace(raw)
}

// NormalizeEmail returns the canonical form of an admin login identifier.
func NormalizeEmail(raw string) string {
	return strings.ToLower(CollapseSpace(raw))
}

// ValidIndexNumberRunes reports whether r may appear in a normalised index
// number. The set is deliberately generous (letters, digits, a few common
// separators) while excluding anything that could be used to smuggle markup or
// control characters.
func ValidIndexNumberRunes(r rune) bool {
	switch {
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	case r == ' ' || r == '/' || r == '-' || r == '.' || r == '_':
		return true
	}
	return false
}

// ValidateStudentInput normalises and validates the two student-supplied
// fields, returning canonical values or a 422 error with field messages.
func ValidateStudentInput(fullName, indexNumber string) (string, string, *Error) {
	name := NormalizeName(fullName)
	index := NormalizeIndexNumber(indexNumber)

	fields := map[string]string{}

	switch {
	case name == "":
		fields["full_name"] = "Full name is required."
	case utf8.RuneCountInString(name) > MaxNameRunes:
		fields["full_name"] = "Full name is too long (max 120 characters)."
	case !hasLetter(name):
		fields["full_name"] = "Full name must contain letters."
	}

	switch {
	case index == "":
		fields["index_number"] = "Index number is required."
	case utf8.RuneCountInString(index) > MaxIndexNumberRunes:
		fields["index_number"] = "Index number is too long (max 40 characters)."
	case !hasDigitOrLetter(index):
		fields["index_number"] = "Index number must contain at least one letter or digit."
	case !onlyValidIndexRunes(index):
		fields["index_number"] = "Index number may only contain letters, digits, spaces and / - . _ characters."
	}

	if len(fields) > 0 {
		return "", "", ValidationError("Please correct the highlighted fields.", fields)
	}
	return name, index, nil
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func hasDigitOrLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func onlyValidIndexRunes(s string) bool {
	for _, r := range s {
		if !ValidIndexNumberRunes(r) {
			return false
		}
	}
	return true
}

// ValidateTopicInput validates admin-supplied topic fields.
func ValidateTopicInput(name, description string, capacity int) (string, string, *Error) {
	cleanName := CollapseSpace(name)
	cleanDesc := strings.TrimSpace(description)

	fields := map[string]string{}
	switch {
	case cleanName == "":
		fields["name"] = "Topic name is required."
	case utf8.RuneCountInString(cleanName) > 120:
		fields["name"] = "Topic name is too long (max 120 characters)."
	}
	switch {
	case utf8.RuneCountInString(cleanDesc) > 2000:
		fields["description"] = "Description is too long (max 2000 characters)."
	}
	if capacity <= 0 {
		fields["capacity"] = "Capacity must be greater than zero."
	} else if capacity > 1000 {
		fields["capacity"] = "Capacity may not exceed 1000."
	}

	if len(fields) > 0 {
		return "", "", ValidationError("Please correct the highlighted fields.", fields)
	}
	return cleanName, cleanDesc, nil
}

// Pagination

const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// Page describes an offset/limit window.
type Page struct {
	Limit  int
	Offset int
}

// Normalize clamps the window into supported bounds.
func (p Page) Normalize() Page {
	if p.Limit <= 0 {
		p.Limit = DefaultPageSize
	}
	if p.Limit > MaxPageSize {
		p.Limit = MaxPageSize
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}

// ListResult is the standard paginated envelope.
type ListResult[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// NewListResult builds a result with a non-nil Items slice so JSON renders
// `[]` rather than `null`.
func NewListResult[T any](items []T, total int, p Page) ListResult[T] {
	if items == nil {
		items = []T{}
	}
	return ListResult[T]{Items: items, Total: total, Limit: p.Limit, Offset: p.Offset}
}
