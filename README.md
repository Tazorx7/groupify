# Groupify

**Pick your topic. Secure your spot.**

Students register for exactly one topic; a course representative configures
topics and capacities, opens and closes the registration period, and manages the
resulting list. The system never allows a topic to exceed its capacity — not
even when two students submit for the last seat at the same instant.

* Product requirements: [`PRD.md`](./PRD.md)
* API reference: [`docs/api.md`](./docs/api.md)
* Schema and concurrency design: [`docs/database.md`](./docs/database.md)
* Deployment: [`docs/deployment.md`](./docs/deployment.md)

## Status

| Milestone (PRD §20) | State |
|---|---|
| 1. Foundation: schema, migrations, config, health | ✅ |
| 2. Core registration: atomic capacity, uniqueness, idempotency | ✅ |
| 3. Student interface + admin UI | ✅ |
| 4. Admin foundation: auth, topic management, open/close | ✅ |
| 5. Admin operations: search, filters, CSV/XLSX export, audit log | ✅ |
| 6. Hardening: concurrency tests, security review, single-origin deploy | ✅ |

## Stack

**Backend** — Go 1.27 · chi · pgx/v5 · PostgreSQL 18 · `log/slog` · Argon2id ·
excelize (XLSX).

**Frontend** — React 19 · TypeScript · Vite · Tailwind CSS v4 · shadcn/ui
(Base UI primitives) · TanStack Query · React Router.

One image serves both: the Go API also serves the built SPA, so `/api` and the
app share an origin (no CORS, first-party cookies).

## Quick start

### Everything in containers

```sh
podman compose up --build     # or: docker compose up --build
# http://localhost:8080       (student page)
# http://localhost:8080/admin (organiser console)
```

`compose.yaml` runs PostgreSQL and the single-origin image with `APP_ENV=dev`.
The image itself is production-configured and works either way: the session
cookie is marked `Secure` per request, so plain-HTTP localhost still works while
HTTPS deployments get the flag. See [`docs/deployment.md`](./docs/deployment.md).

### Local development

```sh
cp .env.example .env      # then adjust if needed
make db-up                # PostgreSQL 18 in a rootless podman container on :5433
make seed                 # creates the admin from ADMIN_EMAIL / ADMIN_PASSWORD
make api                  # http://localhost:8081 (HTTP_ADDR)
cd frontend && npm ci && npm run dev   # http://localhost:5173, proxies /api -> :8081
```

`make db-up` prefers podman (rootless) and falls back to docker. It runs
PostgreSQL on host port **5433** because a system PostgreSQL usually owns 5432.
The checked-in `.env` for this machine sets `HTTP_ADDR=:8081` because 8080 was
already occupied; `.env.example` uses the default `:8080`. The Vite dev server
proxies `/api` to `127.0.0.1:8081`, so nothing is served cross-origin in
development either.

Migrations run automatically on start. `STATIC_DIR` stays empty in development —
the API then serves only JSON and Vite serves the app.

### Build the production shape

```sh
make web      # npm ci && vite build -> frontend/dist
make image    # podman/docker build -t groupify -f Dockerfile .
```

### Try it

```sh
curl localhost:8081/api/registration-status
curl -c jar -X POST localhost:8081/api/admin/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"rep@example.edu","password":"change-me-please-123"}'
./scripts/smoke.sh --reset   # 48 checks; wipes the dev database first
```

## Tests

```sh
make test                 # Go unit tests, no database needed
make db-up && make test-integration   # full Go suite, uses groupify_test
cd backend && TEST_DATABASE_URL=... go test ./internal/registrations -race
cd frontend && npm run build && npm run lint
```

Database-backed tests skip themselves when `TEST_DATABASE_URL` is unset. CI runs
both halves (`.github/workflows/ci.yml`) and both are required checks on `main`.

The Go suite includes the proofs the PRD asks for:

* 40 concurrent registrations against a 5-seat topic → exactly 5 succeed,
  counter equals row count, losers consume nothing.
* 20 concurrent submissions of the same index number → exactly one row, one slot.
* Registration racing an admin close, 25 rounds → no registration ever commits
  after the close returns, and `current_members` always equals the row count.
* A failed insert after the capacity increment rolls the increment back.

The frontend was verified by driving it in a real browser against the live API —
student flow (full-topic lockout, index normalisation, duplicate rejection,
confirmation surviving refresh) and organiser console (auth gate, topic CRUD,
capacity floor, search, CSV/XLSX download, delete, open/close, audit log).

## Layout

```
backend/
  cmd/api/          HTTP entrypoint, graceful shutdown, session GC
  cmd/seed/         bootstrap/dedupe the admin account
  internal/
    config/         env parsing, dotenv loading, fail-fast validation
    database/       pool, transaction helper, embedded-SQL migration runner
    domain/         shared errors, normalisation rules, pagination
    auth/           Argon2id hashing, opaque sessions
    audit/          in-transaction audit writes, admin log reader
    topics/         service · store · http
    registrations/  service (the transaction) · store · http · CSV + XLSX export
    settings/       open/close switch and its lock protocol
    admin/          login, logout, overview
    httpapi/        the single route table and its middleware stack
    static/         serves the built SPA with SPA fallback + cache headers
    middleware/     logging, recovery, security headers, CORS, rate limits, auth
    httpx/          JSON envelopes, strict decoding, error→HTTP mapping
  migrations/       embedded SQL, applied in lexical order
frontend/
  src/
    features/student/   landing, topic cards, form, review, confirmation
    features/admin/     login, overview, topics, registrations, settings, audit
    components/ui/      shadcn/ui primitives
    lib/                typed API client, types, formatting
Dockerfile          single-origin image (SPA build + Go build + distroless)
compose.yaml        db + app for a local full stack
scripts/smoke.sh    end-to-end API check against a running instance
docs/               api · database · deployment
```

## Design decisions worth knowing

* **Capacity is a row lock, not a count.** `UPDATE topics … WHERE current_members
  < capacity RETURNING` inside the registration transaction; `CHECK` constraints
  as the final defence.
* **Close and register linearise** on the singleton settings row
  (`FOR SHARE` vs `FOR UPDATE`). Boundary: committed-before-close survives.
* **One registration per student** is a UNIQUE constraint on the normalised index
  number, not an application check.
* **No student self-service lookup.** Without a roster or one-time code there is
  no safe way to prove a caller owns an index number, so confirmation exists only
  in the POST response (persisted in `sessionStorage` so a refresh does not look
  like a failed registration).
* **Admin removals are the only way to undo a selection**, and they free the slot
  and write an audit row in the same transaction.
* **Audit logs carry no student identifiers** — registration id and topic only.
* **Exports come in CSV and XLSX.** XLSX is grouped: a `Summary` sheet (per topic,
  with capacity/registered/remaining and totals) plus a roster sheet ordered by
  topic. Identifiers are text-typed cells, otherwise Excel turns `00123456` into
  `123456` and quietly corrupts the roster.
* **Retries are safe.** The student page reuses one `Idempotency-Key` per payload,
  so a dropped response cannot register twice.
