# Deployment

## Shape

```
browser ──HTTPS──> reverse proxy / static host ──> frontend (static files)
                            │
                            └──/api──> Go API (one small instance) ──> managed PostgreSQL
```

The API is stateless apart from the database, so it can be scaled — with one
caveat about rate limiting, below.

## Environment variables

| Variable | Required | Default | Notes |
|---|---|---|---|
| `APP_ENV` | – | `dev` | `production` forces secure cookies and JSON logs. |
| `HTTP_ADDR` | – | `:8080` | |
| `DATABASE_URL` | **yes** | – | `postgres://user:pass@host:5432/db?sslmode=require` in production. |
| `TEST_DATABASE_URL` | tests only | – | Separate database; it is truncated between tests. |
| `AUTO_MIGRATE` | – | `true` | Run migrations at start. |
| `SESSION_TTL` | – | `12h` | Sliding window; `last_seen_at` writes are throttled to 1/min. |
| `COOKIE_SECURE` | – | `false` | Forced to `true` when `APP_ENV=production`. |
| `COOKIE_DOMAIN` | – | – | Set only when the API is on a different host than the SPA. |
| `CORS_ORIGINS` | – | – | Comma-separated. Leave empty when the SPA is same-origin (recommended). |
| `TRUST_PROXY_HEADERS` | – | `false` | Set `true` only behind a proxy you control; it enables `X-Forwarded-For` for client IPs. |
| `RATE_LIMIT_LOGIN_PER_MINUTE` | – | `5` | Per client IP. |
| `RATE_LIMIT_REGISTER_PER_MINUTE` | – | `60` | Deliberately generous: a campus NAT presents one IP for a whole lab. |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | seed only | – | Read by `cmd/seed`, never by the API. |

No secret is ever read from the repository. `.env` is git-ignored; production
values come from the platform's secret manager.

## Build

```sh
make build                 # backend/bin/groupify-api
```

Container image (multi-stage, non-root):

```sh
podman build -t groupify-api -f backend/Dockerfile backend
podman run -p 8080:8080 -e DATABASE_URL=... -e APP_ENV=production groupify-api
```

Migrations run inside the container on start (`AUTO_MIGRATE=true`). For
zero-downtime rollouts, set `AUTO_MIGRATE=false` on the API and run
`groupify-api`'s sibling migration step as a release job — the migration runner
serialises on a PostgreSQL advisory lock, so concurrent starts are safe either
way.

## First deploy checklist

1. Create the database and a least-privilege role (owner of the schema; no
   superuser, no `CREATEDB`).
2. Set every variable above from the secret manager.
3. Run the migration/seed step once to create the administrator
   (`ADMIN_EMAIL`, `ADMIN_PASSWORD` — change it after first login).
4. Verify `GET /readyz` returns `200`.
5. Serve the SPA over HTTPS on the same origin as `/api` so session cookies stay
   first-party; that removes the need for `CORS_ORIGINS` entirely.

## Security notes

* Passwords: Argon2id, 64 MiB / t=3 / p=4, PHC-encoded. Login performs a dummy
  verification for unknown accounts so response time does not reveal which
  addresses exist.
* Sessions: opaque 32-byte tokens, SHA-256 stored server-side, so a database dump
  cannot be replayed as a session. Logout deletes the row.
* CSRF: `SameSite=Lax` plus a per-session double-submit token required on every
  mutating admin request.
* Rate limits are in-process. With **more than one API replica** the effective
  limit is per replica, and login throttling is weaker. Keep the API at one
  instance, or put limits at the edge proxy. Documented here rather than
  pretending otherwise.
* Logs contain method, route, status, duration and request id. No names, no index
  numbers, no bodies.
* Audit rows never contain student identifiers.

## Backups

`pg_dump` on a schedule is enough for this data volume; the only
non-regenerable content is `registrations` and `topics`. Restoring a backup
restores `topics.current_members` together with the rows, so the counter stays
consistent.
