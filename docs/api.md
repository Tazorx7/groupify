# Groupify API

Base URL: `http://localhost:8081` in development (see `HTTP_ADDR`), `/api` prefix
for all application endpoints.

All responses are JSON. All errors use one envelope:

```json
{ "error": { "code": "topic_full", "message": "This topic is full. Please choose another topic." } }
```

Validation errors add per-field messages:

```json
{ "error": { "code": "validation_failed", "message": "Please correct the highlighted fields.",
             "fields": { "index_number": "Index number is required." } } }
```

`message` is always safe to display to a user. `code` is stable and is what the
frontend should branch on.

---

## Public / student endpoints

No authentication. These are the only endpoints that expose topic data, and they
return aggregates only — never names, index numbers or membership.

### `GET /api/topics`

Active topics with availability.

```json
{ "topics": [
  { "id": 3, "name": "Distributed Systems", "description": "Consensus and replication",
    "capacity": 25, "registered_count": 9, "remaining_spaces": 16, "status": "available" }
]}
```

`status`: `available` | `full` (`unavailable` is reserved for a topic that is
inactive; inactive topics are not listed).

### `GET /api/registration-status`

```json
{ "registration_open": true }
```

### `POST /api/registrations`

Creates the caller's single registration. Rate limited per IP.

```json
{ "full_name": "Ama Mensah", "index_number": "cs/2024/045", "topic_id": 3 }
```

Optional header `Idempotency-Key: <8-128 chars>` makes retries safe after a
dropped connection.

`201 Created`

```json
{ "registration_id": 42, "full_name": "Ama Mensah", "index_number": "CS/2024/045",
  "topic": { "id": 3, "name": "Distributed Systems" },
  "registered_at": "2026-11-11T09:14:02.113Z" }
```

`full_name` has whitespace collapsed; `index_number` is normalised (trimmed,
internal whitespace collapsed, upper-cased) and this normalised value is what
uniqueness is enforced on.

A replayed idempotent request returns `201` with the original body and the header
`Idempotent-Replayed: true`.

Errors: `409 registration_closed`, `409 topic_full`, `409 topic_inactive`,
`404 topic_not_found`, `409 index_number_already_registered`,
`422 validation_failed`, `422 idempotency_key_reuse`,
`409 idempotency_key_in_progress`, `429 rate_limited`.

There is deliberately **no** `GET /api/registrations/me`. Without a preloaded
roster or a one-time code there is no way to verify that a caller owns an index
number, so any such lookup would be a public record search (see PRD §14).

---

## Admin endpoints

Authentication is an opaque session token in an `HttpOnly`, `SameSite=Lax`
cookie (`groupify_admin`). Every mutating request must also send
`X-CSRF-Token` with the value returned by login or `GET /api/admin/session`.

Unauthenticated ⇒ `401 authentication_required`. Missing/invalid CSRF token on a
mutation ⇒ `403 csrf_check_failed`.

### `POST /api/admin/login`

```json
{ "email": "rep@example.edu", "password": "…" }
```

```json
{ "admin": { "id": 1, "email": "rep@example.edu" },
  "csrf_token": "…", "expires_at": "2026-11-11T21:14:02Z" }
```

Sets the session cookie. Unknown email and wrong password are indistinguishable
(same code, same timing).

### `GET /api/admin/session`

Returns the same shape as login minus the cookie. The SPA calls this on load to
recover the CSRF token after a refresh.

### `POST /api/admin/logout`

Revokes the session server-side and clears the cookie. `204`.

### `GET /api/admin/overview`

```json
{ "total_registrations": 84, "total_topics": 6, "active_topics": 5, "full_topics": 1,
  "total_available_spaces": 61, "registration_open": true }
```

### Topics

| Method | Path | Body | Notes |
|---|---|---|---|
| `GET` | `/api/admin/topics` | — | All topics, active first. |
| `POST` | `/api/admin/topics` | `{name, description, capacity}` | `201`. Name unique case-insensitively. |
| `PATCH` | `/api/admin/topics/{id}` | any of `{name, description, capacity}` | Partial. |
| `POST` | `/api/admin/topics/{id}/activate` | — | |
| `POST` | `/api/admin/topics/{id}/deactivate` | — | Existing registrations survive. |

`capacity` must be `1..1000` and may never be lower than the current occupancy
(`409 capacity_below_members`). `409 topic_name_taken` for duplicates.

### Registrations

`GET /api/admin/registrations?search=&topic_id=&sort=&dir=&limit=&offset=`

`sort`: `registered_at` (default) | `full_name` | `index_number` | `topic`.
`dir`: `desc` (default) | `asc`. `limit` 1–200, default 50.

```json
{ "items": [ { "id": 42, "full_name": "Ama Mensah", "index_number": "CS/2024/045",
               "topic_id": 3, "topic_name": "Distributed Systems",
               "created_at": "2026-11-11T09:14:02Z" } ],
  "total": 84, "limit": 50, "offset": 0 }
```

`search` matches full name (case-insensitive substring) or normalised index
number.

`GET /api/admin/registrations/export?…same filters…` → `text/csv`, UTF-8 with
BOM, columns `index_number,full_name,topic,registered_at`. Fields that begin with
`= + - @` are prefixed with `'` to neutralise spreadsheet formula injection. Hard
cap 10 000 rows.

`DELETE /api/admin/registrations/{id}` → `204`. Removes the registration and
releases the capacity slot in one transaction. This is the only sanctioned way to
undo a confirmed selection; it is always audited.

### Settings

| Method | Path | Body |
|---|---|---|
| `GET` | `/api/admin/settings` | — |
| `PATCH` | `/api/admin/settings` | `{ "registration_open": true }` |

`GET /api/admin/settings` returns `{registration_open, updated_at, updated_by_admin_id}`.

### `GET /api/admin/audit-logs?action=&entity_type=&limit=&offset=`

Newest first. Each entry: `id, actor_admin_id, actor_email, actor_label, action,
entity_type, entity_id, before_data, after_data, created_at`.

Actions: `registration.created`, `registration.deleted`,
`registration.period.opened`, `registration.period.closed`, `topic.created`,
`topic.updated`, `topic.capacity_changed`, `topic.activated`,
`topic.deactivated`, `admin.login`, `admin.logout`.

Audit rows never contain student names or index numbers.

---

## Operational endpoints

| Path | Purpose |
|---|---|
| `GET /healthz` | Liveness. Always `200` if the process is up. |
| `GET /readyz` | Readiness. `200` when the database answers, `503` otherwise. |

---

## Status codes used

`200` ok · `201` created · `204` no content · `400` malformed request ·
`401` authentication required · `403` CSRF or permission · `404` unknown
resource · `409` business conflict (closed, full, inactive, duplicate) ·
`413` body too large · `422` validation · `429` rate limited · `500` unexpected.
