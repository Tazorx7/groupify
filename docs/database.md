# Database design

PostgreSQL 18. `backend/migrations` is the only source of truth for schema and
is applied automatically at API start (`AUTO_MIGRATE=true`) or via
`cmd/seed`/the migration runner, which serialises on an advisory lock.

## Tables

```
topics (id, name, description, capacity, current_members, is_active, created_at, updated_at)
registrations (id, full_name, index_number, topic_id → topics, created_at)
admins (id, email, password_hash, created_at, updated_at)
admin_sessions (token_hash, admin_id → admins, csrf_token, created_at, last_seen_at, expires_at)
app_settings (id=1, registration_open, updated_at, updated_by_admin_id → admins)
audit_logs (id, actor_admin_id → admins, actor_label, action, entity_type, entity_id,
            before_data jsonb, after_data jsonb, request_id, created_at)
idempotency_keys (key, request_hash, response_status, response_body jsonb, created_at)
```

## Why `current_members` exists

Reading `count(*)` from `registrations` on every topic view is correct but does
not give you a place to make the capacity decision atomically. The counter is a
row-level resource that PostgreSQL can serialise on:

```sql
UPDATE topics
SET current_members = current_members + 1
WHERE id = $1 AND is_active AND current_members < capacity
RETURNING current_members, capacity, name;
```

Zero rows means "no seat for you" — inactive, full, or gone. The application then
issues a second, non-locking `SELECT` purely to pick the right error message.

The counter is only ever mutated by code that also mutates the matching
`registrations` row, inside one transaction. Two `CHECK` constraints keep the
data honest if that ever slips:

```sql
CHECK (current_members >= 0)
CHECK (current_members <= capacity)
```

`TestDatabaseRejectsOverbooking` asserts both.

## Constraints

| Constraint | Purpose |
|---|---|
| `registrations.index_number` UNIQUE | One registration per student, enforced under concurrency. |
| `topics.name` UNIQUE on `lower(btrim(name))` | No duplicate topics. |
| `topics.capacity > 0`, `<= 1000` (app) | Sane capacities. |
| `current_members >= 0` and `<= capacity` | Final defence against overbooking. |
| `registrations.topic_id → topics ON DELETE RESTRICT` | A topic with members cannot be deleted. |

## Index number normalisation

Index numbers are strings. The storage form is:

1. trim surrounding whitespace (including NBSP / zero-width space),
2. collapse internal whitespace runs to one ASCII space,
3. upper-case (Unicode simple mapping).

So `" cs/2024/045 "` and `"CS/2024/045"` are the same student. Allowed
characters after normalisation: letters, digits, space, `/ - . _`. Length 1–40.
The same normalisation is applied to admin search terms.

## The concurrency protocol

Two locks, both taken inside the registration transaction:

```sql
BEGIN;                                              -- READ COMMITTED

SELECT registration_open FROM app_settings
WHERE id = 1 FOR SHARE;                             -- 1. admit / reject

-- optional: friendlier duplicate error
SELECT EXISTS (SELECT 1 FROM registrations WHERE index_number = $2);

UPDATE topics SET current_members = current_members + 1
WHERE id = $1 AND is_active AND current_members < capacity
RETURNING ...;                                      -- 2. seat

INSERT INTO registrations (full_name, index_number, topic_id)
VALUES ($3, $2, $1) RETURNING ...;                  -- 3. record

-- idempotency row completed, audit row written
COMMIT;
```

* Step 2 relies on PostgreSQL's row-level lock plus EvalPlanQual re-evaluation:
  a transaction that loses the race re-checks `current_members < capacity`
  against the committed row and gets zero rows. No `SERIALIZABLE` needed.
* Step 3's UNIQUE violation aborts the transaction, which undoes step 2. That is
  why a duplicate never consumes a seat (`TestFailedInsertReleasesCapacitySlot`).
* `FOR SHARE` on the settings row is compatible with other registrations (they
  all take SHARE) but conflicts with the admin close path, which takes
  `FOR UPDATE` on the same row. The two operations therefore linearise:
  either the registration commits first and stays valid, or it waits, re-reads
  `false`, and is rejected. `TestCloseRegistrationRace` asserts this 25 times
  over.

Lock ordering is **settings → topics → registrations** everywhere. Admin
removal locks the topic row before deleting the registration row, matching that
order, so the two paths cannot deadlock.

## Idempotency

`idempotency_keys` is claimed with `INSERT … ON CONFLICT (key) DO NOTHING
RETURNING key` as the *first* statement of the transaction. A concurrent retry
therefore blocks on the uncommitted row instead of racing, and once the first
transaction commits it reads the stored response:

* same key, same payload → replay the original `201` body,
* same key, different payload → `422 idempotency_key_reuse`,
* same key, first request still running → `409 idempotency_key_in_progress`.

The unique index on `index_number` still protects clients that send no key at
all.

## Audit log

Written in the same transaction as the mutation it describes, so the log cannot
disagree with the data. `before_data` / `after_data` are JSONB. Student
identifiers are **not** recorded: a registration event stores
`registration_id`, `topic_id` and topic name only.

## Test isolation

`TEST_DATABASE_URL` points at a separate database (`groupify_test`). Each test
truncates all tables and re-inserts the `app_settings` singleton. Run the suite
with `-p 1` so packages do not truncate each other's data.
