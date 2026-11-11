package registrations_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/internal/registrations"
	"github.com/groupify/groupify/backend/internal/settings"
	"github.com/groupify/groupify/backend/internal/testsupport"
)

// TestConcurrentRegistrationsNeverExceedCapacity is the headline proof for
// PRD §12: with 5 seats and 40 simultaneous requests, exactly 5 succeed, no
// failed request consumes a slot, and the counter matches the row count.
func TestConcurrentRegistrationsNeverExceedCapacity(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	topicID := testsupport.SeedTopic(t, pool, "Small Room", 5, true)
	testsupport.SetRegistrationOpen(t, pool, true)

	const attempts = 40

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		success  int
		rejected = map[string]int{}
	)

	start := make(chan struct{})
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // maximise overlap

			_, err := svc.Register(context.Background(), registrations.CreateRequest{
				FullName:    fmt.Sprintf("Student %02d", i),
				IndexNumber: fmt.Sprintf("RACE-%03d", i),
				TopicID:     topicID,
			}, "", "")

			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				success++
			case errors.Is(err, domain.ErrTopicFull):
				rejected["topic_full"]++
			default:
				rejected["other:"+err.Error()]++
			}
		}(i)
	}

	close(start)
	wg.Wait()

	if success != 5 {
		t.Fatalf("successful registrations = %d, want exactly 5 (%v)", success, rejected)
	}
	if rejected["topic_full"] != attempts-5 {
		t.Fatalf("topic_full rejections = %d, want %d (%v)", rejected["topic_full"], attempts-5, rejected)
	}
	for k := range rejected {
		if k != "topic_full" {
			t.Fatalf("unexpected failure mode %q: successful sign-ups must only lose to a full topic", k)
		}
	}

	if members := testsupport.TopicMembers(t, pool, topicID); members != 5 {
		t.Fatalf("current_members = %d, want 5", members)
	}
	if rows := testsupport.CountRegistrations(t, pool); rows != 5 {
		t.Fatalf("registration rows = %d, want 5", rows)
	}
}

// Concurrent submissions of the same index number must produce exactly one
// registration and consume exactly one slot.
func TestConcurrentDuplicateIndexOnlyOneWins(t *testing.T) {
	pool := testsupport.DB(t)
	svc := registrations.NewService(pool)
	a := testsupport.SeedTopic(t, pool, "Topic A", 10, true)
	b := testsupport.SeedTopic(t, pool, "Topic B", 10, true)
	testsupport.SetRegistrationOpen(t, pool, true)

	const attempts = 20

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		success int
	)

	start := make(chan struct{})
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start

			topicID := a
			if i%2 == 1 {
				topicID = b
			}
			// Vary case and padding: normalisation must still collapse them.
			index := []string{"DUP-001", "dup-001", " DUP-001 ", "Dup-001"}[i%4]

			_, err := svc.Register(context.Background(), registrations.CreateRequest{
				FullName:    fmt.Sprintf("Student %02d", i),
				IndexNumber: index,
				TopicID:     topicID,
			}, "", "")

			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}(i)
	}

	close(start)
	wg.Wait()

	if success != 1 {
		t.Fatalf("successful registrations = %d, want exactly 1", success)
	}
	if rows := testsupport.CountRegistrations(t, pool); rows != 1 {
		t.Fatalf("registration rows = %d, want 1", rows)
	}
	total := testsupport.TopicMembers(t, pool, a) + testsupport.TopicMembers(t, pool, b)
	if total != 1 {
		t.Fatalf("total current_members = %d, want 1 (losers must not consume slots)", total)
	}
}

// TestCloseRegistrationRace asserts the documented boundary from PRD §12:
// a registration that committed before closure stands, and once Close has
// returned no further registration can commit.
func TestCloseRegistrationRace(t *testing.T) {
	pool := testsupport.DB(t)
	regSvc := registrations.NewService(pool)
	settingsSvc := settings.NewService(pool)
	topicID := testsupport.SeedTopic(t, pool, "Race Topic", 100, true)
	adminID := testsupport.SeedAdmin(t, pool, "rep@example.edu", "a-long-password-1")

	const rounds = 25

	for round := 0; round < rounds; round++ {
		testsupport.SetRegistrationOpen(t, pool, true)

		var wg sync.WaitGroup
		start := make(chan struct{})

		wg.Add(2)
		go func(r int) {
			defer wg.Done()
			<-start
			_, _ = regSvc.Register(context.Background(), registrations.CreateRequest{
				FullName:    "Racing Student",
				IndexNumber: fmt.Sprintf("CLOSE-%03d", r),
				TopicID:     topicID,
			}, "", "")
		}(round)
		go func() {
			defer wg.Done()
			<-start
			if _, err := settingsSvc.SetOpen(context.Background(), adminID, false, "req-close"); err != nil {
				t.Errorf("close: %v", err)
			}
		}()
		close(start)
		wg.Wait()

		// After Close has returned, the switch must be closed...
		st, err := settingsSvc.Get(context.Background())
		if err != nil {
			t.Fatalf("read settings: %v", err)
		}
		if st.RegistrationOpen {
			t.Fatalf("round %d: registration still open after close returned", round)
		}

		// ...and a brand-new registration must be rejected.
		_, err = regSvc.Register(context.Background(), registrations.CreateRequest{
			FullName:    "Late Student",
			IndexNumber: fmt.Sprintf("LATE-%03d", round),
			TopicID:     topicID,
		}, "", "")
		if !errors.Is(err, domain.ErrRegistrationClosed) {
			t.Fatalf("round %d: err = %v, want ErrRegistrationClosed", round, err)
		}
	}

	// Invariant across all rounds: occupancy never exceeds the number of rows.
	rows := testsupport.CountRegistrations(t, pool)
	members := testsupport.TopicMembers(t, pool, topicID)
	if members != rows {
		t.Fatalf("current_members = %d but rows = %d; a slot leaked or was double counted", members, rows)
	}
	if members > rounds {
		t.Fatalf("current_members = %d, want at most %d", members, rounds)
	}
}

// Admission under a closing switch is all-or-nothing: every committed
// registration corresponds to exactly one consumed slot.
func TestCounterMatchesRowsAfterMixedLoad(t *testing.T) {
	pool := testsupport.DB(t)
	regSvc := registrations.NewService(pool)

	const topics = 4
	ids := make([]int64, 0, topics)
	for i := 0; i < topics; i++ {
		ids = append(ids, testsupport.SeedTopic(t, pool, fmt.Sprintf("Topic %d", i), 10, true))
	}
	testsupport.SetRegistrationOpen(t, pool, true)

	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = regSvc.Register(context.Background(), registrations.CreateRequest{
				FullName:    fmt.Sprintf("Student %02d", i),
				IndexNumber: fmt.Sprintf("MIX-%03d", i),
				TopicID:     ids[i%topics],
			}, "", "")
		}(i)
	}
	wg.Wait()

	var totalMembers int
	for _, id := range ids {
		totalMembers += testsupport.TopicMembers(t, pool, id)
	}
	rows := testsupport.CountRegistrations(t, pool)
	if totalMembers != rows {
		t.Fatalf("sum(current_members) = %d but rows = %d", totalMembers, rows)
	}
	if rows != 40 {
		t.Fatalf("rows = %d, want 40 (capacity was ample)", rows)
	}
}
