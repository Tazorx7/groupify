# Deployment

## Shape

One image, one origin.

```
browser ──HTTPS──> reverse proxy ──> groupify (Go API + built SPA) ──> managed PostgreSQL
```

The Go binary serves both `/api/*` and the built single-page application
(`STATIC_DIR`). Because the browser sees a single origin, the session cookie is
first-party, no `CORS_ORIGINS` is needed, and there is no static-host rewrite
rule to keep in step with `react-router` routes.

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
| `STATIC_DIR` | – | – | Directory holding the built SPA (`index.html` + `assets/`). The image sets this to `/app/static`; leave empty in development so the Vite dev server proxies `/api` instead. The API refuses to start if it is set but has no `index.html`. |
| `SESSION_TTL` | – | `12h` | Sliding window; `last_seen_at` writes are throttled to 1/min. |
| `COOKIE_SECURE` | – | `auto` | `auto` marks the cookie Secure whenever the request arrived over HTTPS (directly or via a trusted `X-Forwarded-Proto`). `always` forces it. `never` is refused when `APP_ENV=production`. |
| `COOKIE_DOMAIN` | – | – | Only needed when the API is on a different host than the SPA — not the case for the single-origin image. |
| `CORS_ORIGINS` | – | – | Comma-separated. Leave empty: the SPA is same-origin, so no `Access-Control-*` headers are emitted. |
| `TRUST_PROXY_HEADERS` | – | `false` | Set `true` only behind a proxy you control; it enables `X-Forwarded-For` (client IPs) and `X-Forwarded-Proto` (the cookie's Secure flag). |
| `RATE_LIMIT_LOGIN_PER_MINUTE` | – | `5` | Per client IP. |
| `RATE_LIMIT_REGISTER_PER_MINUTE` | – | `60` | Deliberately generous: a campus NAT presents one IP for a whole lab. |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | seed only | – | Read by `cmd/seed`, never by the API. |

No secret is ever read from the repository. `.env` is git-ignored; production
values come from the platform's secret manager.

> **Cookies are decided per request.** The Secure flag is not a startup
> boolean: a Secure cookie is never sent over plain HTTP, so marking one
> unconditionally means sign-in "works" and the session silently never returns —
> the most tedious possible bug to diagnose. `COOKIE_SECURE=auto` (the default)
> marks the cookie Secure exactly when the request arrived over HTTPS, so the
> same image behaves correctly behind a TLS proxy, on localhost over HTTP, and
> under `APP_ENV=production`.
>
> Two guards back that up. `APP_ENV=production` refuses to start with
> `COOKIE_SECURE=never`, because the admin session would cross the wire in clear
> text. And in production a session is only issued over HTTPS **or to a loopback
> host** — anyone else hitting a plain-HTTP deployment gets
> `500 insecure_transport` instead of a token, with no cookie written. Loopback
> is allowed precisely so the deployed image can be checked on the machine that
> built it.
>
> Startup logs the resolved policy (`cookie_secure`, `trust_proxy_headers`) and
> warns when production runs with `auto` but `TRUST_PROXY_HEADERS=false`, since
> that combination only marks cookies Secure when TLS terminates on the process
> itself.

## Build

Local binaries:

```sh
make build                 # backend/bin/groupify-api
make web                   # npm ci && vite build -> frontend/dist
```

Single-origin container (multi-stage, non-root, distroless):

```sh
make image                 # podman/docker build -t groupify -f Dockerfile .
podman run -p 8080:8080 \
  -e APP_ENV=production \
  -e DATABASE_URL=postgres://... \
  groupify
```

With `APP_ENV=production` the cookie is `Secure`, so this must sit behind HTTPS
before you use it (see the cookie note above).

`make image` runs two stages — `node:24-alpine` builds the SPA, `golang:1.27-alpine`
builds the API — then copies both into `gcr.io/distroless/static-debian12:nonroot`
with `STATIC_DIR=/app/static` already set.

Run the whole stack locally with Postgres:

```sh
podman compose up --build     # or: docker compose up --build
```

`compose.yaml` starts `db` and then `app` on `http://localhost:8080`, with
`APP_ENV=dev` so cookies work over plain HTTP on the loopback interface.

Migrations run inside the container on start (`AUTO_MIGRATE=true`). For
zero-downtime rollouts, set `AUTO_MIGRATE=false` on the API and run the sibling
migration step as a release job — the migration runner serialises on a PostgreSQL
advisory lock, so concurrent starts are safe either way.

## Serving the SPA

`internal/static` handles every path the router does not claim:

* `/api/*` is matched first, so an unknown API path is still a JSON `404`
  (`{"error":{"code":"not_found",...}}`) — it never falls through to the shell.
* A real file under `STATIC_DIR` is served as-is; anything under `/assets/` gets
  `Cache-Control: public, max-age=31536000, immutable` because Vite hashes those
  filenames.
* Everything else (including `/`, `/admin`, `/admin/registrations`) returns
  `index.html` with `Cache-Control: no-cache`, so a refresh on a deep link works
  and a deploy is picked up immediately.
* Directory listings are never served.

That is the whole rewrite story: one catch-all in Go, nothing at the proxy.

## First deploy checklist

1. Create the database and a least-privilege role (owner of the schema; no
   superuser, no `CREATEDB`).
2. Build and push the single-origin image; set every variable above from the
   secret manager, including `APP_ENV=production`.
3. Terminate HTTPS at the reverse proxy and forward to `HTTP_ADDR`. If the proxy
   sets `X-Forwarded-For`, also set `TRUST_PROXY_HEADERS=true`.
4. Run the seed step once to create the administrator (`ADMIN_EMAIL`,
   `ADMIN_PASSWORD` — change it after first login).
5. Verify `GET /readyz` returns `200`, `GET /` returns the shell, and `GET /api/topics`
   returns JSON.
6. Sign in at `/admin/login` over HTTPS and confirm the session survives a reload.
   The startup log line `cookie_secure=auto trust_proxy_headers=…` tells you
   which policy is in force; `Set-Cookie` on the login response must carry
   `Secure` when the browser is on `https://`.

## Security notes

* The session cookie is `HttpOnly`, `SameSite=Lax`, and `Secure` whenever the
  request arrived over HTTPS (see the note above). Sessions are only issued over
  plain HTTP to loopback in production.
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
* Security headers (nosniff, frame denial, referrer policy) are set by middleware
  for both the API responses and the SPA shell.

## Backups

`pg_dump` on a schedule is enough for this data volume; the only
non-regenerable content is `registrations` and `topics`. Restoring a backup
restores `topics.current_members` together with the rows, so the counter stays
consistent.
