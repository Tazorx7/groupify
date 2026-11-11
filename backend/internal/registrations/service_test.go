package registrations_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/groupify/groupify/backend/internal/database"
	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/internal/registrations"
	"github.com/groupify/groupify/backend/internal/testsupport"
)

func TestRegisterSucceeds(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	topicID := testsupport.SeedTopic(t, pool, "Algorithms", 10, true)
	testsupport.SetRegistrationOpen(t, pool, true)

	ctx := context.Background()
	res, err := svc.Register(ctx, registrations.CreateRequest{
		FullName:    "  Ama   Mensah ",
		IndexNumber: " cs/2024/045 ",
		TopicID:     topicID,
	}, "test-key-0001", "req-1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if res.Status != 201 {
		t.Fatalf("status = %d", res.Status)
	}
	if res.Confirmation.FullName != "Ama Mensah" {
		t.Fatalf("name = %q", res.Confirmation.FullName)
	}
	if res.Confirmation.IndexNumber != "CS/2024/045" {
		t.Fatalf("index = %q", res.Confirmation.IndexNumber)
	}
	if res.Confirmation.Topic.ID != topicID || res.Confirmation.Topic.Name != "Algorithms" {
		t.Fatalf("topic = %+v", res.Confirmation.Topic)
	}
	if n := testsupport.TopicMembers(t, pool, topicID); n != 1 {
		t.Fatalf("current_members = %d, want 1", n)
	}
	if n := testsupport.CountRegistrations(t, pool); n != 1 {
		t.Fatalf("registrations = %d, want 1", n)
	}

	// The audit trail must exist and must not contain student identifiers.
	var blob string
	if err := pool.QueryRow(ctx, `SELECT coalesce(string_agg(coalesce(after_data::text,''), ' '), '') FROM audit_logs`).Scan(&blob); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if strings.Contains(blob, "CS/2024/045") || strings.Contains(blob, "Ama") {
		t.Fatalf("audit log leaked student data: %s", blob)
	}
}

func TestRegisterRejectsDuplicateNormalisedIndex(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	a := testsupport.SeedTopic(t, pool, "Topic A", 5, true)
	b := testsupport.SeedTopic(t, pool, "Topic B", 5, true)
	testsupport.SetRegistrationOpen(t, pool, true)

	ctx := context.Background()
	_, err := svc.Register(ctx, registrations.CreateRequest{FullName: "Ama Mensah", IndexNumber: "00123456", TopicID: a}, "", "")
	if err != nil {
		t.Fatalf("first registration: %v", err)
	}

	// Same index, different topic, different case and spacing.
	_, err = svc.Register(ctx, registrations.CreateRequest{FullName: "Ama M.", IndexNumber: " 00123456 ", TopicID: b}, "", "")
	if !errors.Is(err, domain.ErrIndexNumberTaken) {
		t.Fatalf("err = %v, want ErrIndexNumberTaken", err)
	}

	if n := testsupport.CountRegistrations(t, pool); n != 1 {
		t.Fatalf("registrations = %d, want 1", n)
	}
	if n := testsupport.TopicMembers(t, pool, b); n != 0 {
		t.Fatalf("second topic members = %d, want 0 (no slot consumed)", n)
	}
}

func TestRegisterRejectsWhenClosed(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	topicID := testsupport.SeedTopic(t, pool, "Topic", 5, true)
	testsupport.SetRegistrationOpen(t, pool, false)

	_, err := svc.Register(context.Background(), registrations.CreateRequest{
		FullName: "Ama Mensah", IndexNumber: "1", TopicID: topicID,
	}, "", "")
	if !errors.Is(err, domain.ErrRegistrationClosed) {
		t.Fatalf("err = %v, want ErrRegistrationClosed", err)
	}
	if n := testsupport.CountRegistrations(t, pool); n != 0 {
		t.Fatalf("registrations = %d, want 0", n)
	}
}

func TestRegisterRejectsInactiveTopic(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	topicID := testsupport.SeedTopic(t, pool, "Closed Topic", 5, false)
	testsupport.SetRegistrationOpen(t, pool, true)

	_, err := svc.Register(context.Background(), registrations.CreateRequest{
		FullName: "Ama Mensah", IndexNumber: "1", TopicID: topicID,
	}, "", "")
	if !errors.Is(err, domain.ErrTopicInactive) {
		t.Fatalf("err = %v, want ErrTopicInactive", err)
	}
}

func TestRegisterRejectsFullTopic(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	topicID := testsupport.SeedTopic(t, pool, "Tiny", 1, true)
	testsupport.SetRegistrationOpen(t, pool, true)

	ctx := context.Background()
	if _, err := svc.Register(ctx, registrations.CreateRequest{FullName: "First Student", IndexNumber: "1", TopicID: topicID}, "", ""); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := svc.Register(ctx, registrations.CreateRequest{FullName: "Second Student", IndexNumber: "2", TopicID: topicID}, "", "")
	if !errors.Is(err, domain.ErrTopicFull) {
		t.Fatalf("err = %v, want ErrTopicFull", err)
	}
	if n := testsupport.CountRegistrations(t, pool); n != 1 {
		t.Fatalf("registrations = %d, want 1", n)
	}
}

func TestRegisterRejectsUnknownTopic(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	testsupport.SetRegistrationOpen(t, pool, true)

	_, err := svc.Register(context.Background(), registrations.CreateRequest{
		FullName: "Ama Mensah", IndexNumber: "1", TopicID: 999999,
	}, "", "")
	if !errors.Is(err, domain.ErrTopicNotFound) {
		t.Fatalf("err = %v, want ErrTopicNotFound", err)
	}
}

// A failed INSERT must roll back the capacity increment. This is the exact
// failure mode PRD §12 calls out: "If the insert fails ... the transaction must
// roll back so the capacity counter is not consumed."
func TestFailedInsertReleasesCapacitySlot(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	store := registrations.NewStore(pool)
	topicID := testsupport.SeedTopic(t, pool, "Topic", 5, true)
	testsupport.SetRegistrationOpen(t, pool, true)

	ctx := context.Background()
	if _, err := svc.Register(ctx, registrations.CreateRequest{FullName: "Existing Student", IndexNumber: "TAKEN-1", TopicID: topicID}, "", ""); err != nil {
		t.Fatalf("seed registration: %v", err)
	}
	before := testsupport.TopicMembers(t, pool, topicID)

	// Claim a slot, then fail the insert exactly as a duplicate would.
	err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := store.ClaimSlot(ctx, tx, topicID); err != nil {
			return err
		}
		_, err := store.Insert(ctx, tx, "Duplicate Student", "TAKEN-1", topicID)
		return err
	})
	if !errors.Is(err, domain.ErrIndexNumberTaken) {
		t.Fatalf("err = %v, want ErrIndexNumberTaken", err)
	}

	if after := testsupport.TopicMembers(t, pool, topicID); after != before {
		t.Fatalf("current_members = %d, want %d (slot must be released)", after, before)
	}
	if n := testsupport.CountRegistrations(t, pool); n != 1 {
		t.Fatalf("registrations = %d, want 1", n)
	}
}

// The CHECK constraint is the final line of defence if application logic is
// ever bypassed.
func TestDatabaseRejectsOverbooking(t *testing.T) {
	pool := testsupport.DB(t)
	topicID := testsupport.SeedTopic(t, pool, "Tiny", 2, true)

	_, err := pool.Exec(context.Background(),
		`UPDATE topics SET current_members = capacity + 1 WHERE id = $1`, topicID)
	if err == nil {
		t.Fatal("expected CHECK constraint violation")
	}
	if !database.IsCheckViolation(err, database.ConstraintMembersCapacity) {
		t.Fatalf("err = %v, want topics_members_within_capacity check violation", err)
	}

	_, err = pool.Exec(context.Background(),
		`UPDATE topics SET current_members = -1 WHERE id = $1`, topicID)
	if err == nil {
		t.Fatal("expected CHECK constraint violation for negative members")
	}
}

func TestDuplicateIndexRejectedByDatabaseEvenWithoutService(t *testing.T) {
	pool := testsupport.DB(t)
	topicID := testsupport.SeedTopic(t, pool, "Topic", 5, true)

	_, err := pool.Exec(context.Background(),
		`INSERT INTO registrations (full_name, index_number, topic_id) VALUES ('A', 'X1', $1)`, topicID)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err = pool.Exec(context.Background(),
		`INSERT INTO registrations (full_name, index_number, topic_id) VALUES ('B', 'X1', $1)`, topicID)
	if !database.IsUniqueViolation(err, database.ConstraintIndexNumberUnique) {
		t.Fatalf("err = %v, want unique violation", err)
	}
}

func TestIdempotentRetryReturnsSameRegistration(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	topicID := testsupport.SeedTopic(t, pool, "Topic", 5, true)
	testsupport.SetRegistrationOpen(t, pool, true)

	ctx := context.Background()
	req := registrations.CreateRequest{FullName: "Ama Mensah", IndexNumber: "IDEM-1", TopicID: topicID}

	first, err := svc.Register(ctx, req, "idem-key-123456", "req-1")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.Register(ctx, req, "idem-key-123456", "req-2")
	if err != nil {
		t.Fatalf("retry: %v", err)
	}

	if !second.Replayed {
		t.Fatal("expected the retry to be a replay")
	}
	if second.Confirmation.RegistrationID != first.Confirmation.RegistrationID {
		t.Fatalf("replay id = %d, want %d", second.Confirmation.RegistrationID, first.Confirmation.RegistrationID)
	}
	if n := testsupport.CountRegistrations(t, pool); n != 1 {
		t.Fatalf("registrations = %d, want 1", n)
	}
	// Critically: the retry must not consume a second slot.
	if n := testsupport.TopicMembers(t, pool, topicID); n != 1 {
		t.Fatalf("current_members = %d, want 1", n)
	}
}

func TestIdempotencyKeyReuseWithDifferentBody(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	topicID := testsupport.SeedTopic(t, pool, "Topic", 5, true)
	testsupport.SetRegistrationOpen(t, pool, true)

	ctx := context.Background()
	if _, err := svc.Register(ctx, registrations.CreateRequest{FullName: "Ama Mensah", IndexNumber: "K-1", TopicID: topicID}, "reused-key-999", ""); err != nil {
		t.Fatalf("first: %v", err)
	}

	_, err := svc.Register(ctx, registrations.CreateRequest{FullName: "Someone Else", IndexNumber: "K-2", TopicID: topicID}, "reused-key-999", "")
	if !errors.Is(err, domain.ErrIdempotencyReuse) {
		t.Fatalf("err = %v, want ErrIdempotencyReuse", err)
	}
}

func TestAdminDeleteReleasesSlotAndAudits(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	topicID := testsupport.SeedTopic(t, pool, "Topic", 1, true)
	testsupport.SetRegistrationOpen(t, pool, true)
	adminID := testsupport.SeedAdmin(t, pool, "rep@example.edu", "a-long-password-1")

	ctx := context.Background()
	res, err := svc.Register(ctx, registrations.CreateRequest{FullName: "Ama Mensah", IndexNumber: "DEL-1", TopicID: topicID}, "", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := svc.Delete(ctx, adminID, res.Confirmation.RegistrationID, "req-del"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if n := testsupport.CountRegistrations(t, pool); n != 0 {
		t.Fatalf("registrations = %d, want 0", n)
	}
	if n := testsupport.TopicMembers(t, pool, topicID); n != 0 {
		t.Fatalf("current_members = %d, want 0", n)
	}

	// The freed slot must be usable again.
	if _, err := svc.Register(ctx, registrations.CreateRequest{FullName: "Next Student", IndexNumber: "DEL-2", TopicID: topicID}, "", ""); err != nil {
		t.Fatalf("register after delete: %v", err)
	}

	actions := testsupport.AuditActions(t, pool)
	found := false
	for _, a := range actions {
		if a == "registration.deleted" {
			found = true
		}
	}
	if !found {
		t.Fatalf("registration.deleted missing from audit actions: %v", actions)
	}

	// Deleting twice must not double-decrement.
	if err := svc.Delete(ctx, adminID, res.Confirmation.RegistrationID, "req-del-2"); !errors.Is(err, domain.ErrRegistrationNotFound) {
		t.Fatalf("second delete err = %v, want ErrRegistrationNotFound", err)
	}
	if n := testsupport.TopicMembers(t, pool, topicID); n != 1 {
		t.Fatalf("current_members = %d, want 1 after failed second delete", n)
	}
}
