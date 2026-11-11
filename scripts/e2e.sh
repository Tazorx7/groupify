#!/usr/bin/env bash
# Run the Playwright suite against a real, throwaway stack.
#
# The browser tests need the thing that ships: the built SPA served by the Go
# API, with Postgres behind it. This script builds both halves, creates a
# database it owns (and destroys), seeds the admin, starts the API the way
# production would, and runs the suite against it.
#
#   ./scripts/e2e.sh                # whole suite
#   ./scripts/e2e.sh --headed       # watch it
#   ./scripts/e2e.sh student.spec.ts
#
# Requires: go, node, psql, and the dev database container (`make db-up`).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${E2E_PORT:-8090}"
DB_NAME="${E2E_DATABASE:-groupify_e2e}"
BASE_URL="http://127.0.0.1:${PORT}"
ADMIN_EMAIL="${ADMIN_EMAIL:-rep@example.edu}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-change-me-please-123}"

# Reuse the developer's connection details but point at a database this script
# may freely drop.
ADMIN_DATABASE_URL="${DATABASE_URL:-postgres://groupify:groupify@127.0.0.1:5433/groupify?sslmode=disable}"
url_no_query="${ADMIN_DATABASE_URL%%\?*}"
query=""
case "$ADMIN_DATABASE_URL" in
  *\?*) query="?${ADMIN_DATABASE_URL#*\?}" ;;
esac
db_root="${url_no_query%/*}"
DATABASE_URL="${db_root}/${DB_NAME}${query}"
MAINTENANCE_URL="${db_root}/postgres${query}"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/groupify-e2e.XXXXXX")"
API_PID=""

cleanup() {
  local status=$?
  if [ "$status" -ne 0 ] && [ -f "${TMP}/api.log" ]; then
    echo
    echo "--- API log (last 40 lines) ---"
    tail -40 "${TMP}/api.log" || true
  fi
  if [ -n "${API_PID}" ]; then
    kill "${API_PID}" 2>/dev/null || true
    wait "${API_PID}" 2>/dev/null || true
  fi
  rm -rf "${TMP}"
  exit "$status"
}
trap cleanup EXIT

command -v psql >/dev/null || { echo "psql is required (PostgreSQL client)"; exit 1; }

echo "==> rebuilding ${DB_NAME}"
psql "${MAINTENANCE_URL}" -q -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS \"${DB_NAME}\" WITH (FORCE)" \
  -c "CREATE DATABASE \"${DB_NAME}\""

echo "==> building the API"
(cd "${ROOT}/backend" &&
  go build -trimpath -o "${TMP}/groupify-api" ./cmd/api &&
  go build -trimpath -o "${TMP}/groupify-seed" ./cmd/seed)

if [ ! -d "${ROOT}/frontend/node_modules" ]; then
  echo "==> installing frontend dependencies"
  (cd "${ROOT}/frontend" && npm ci)
fi

echo "==> building the SPA"
(cd "${ROOT}/frontend" && npm run build)

echo "==> installing the browser if needed"
(cd "${ROOT}/frontend" && npx playwright install chromium >/dev/null)

# The rate limits are per client IP and the whole suite comes from one: the
# stock 5 sign-ins a minute would fail the run long before it finished testing
# anything. The limiter has its own coverage; this is not a bypass of it in
# production, just of it in a test harness.
echo "==> starting the API on ${BASE_URL} (APP_ENV=production, cookies auto)"
APP_ENV=production \
HTTP_ADDR=":${PORT}" \
STATIC_DIR="${ROOT}/frontend/dist" \
DATABASE_URL="${DATABASE_URL}" \
AUTO_MIGRATE=true \
RATE_LIMIT_LOGIN_PER_MINUTE="${E2E_LOGIN_LIMIT:-200}" \
RATE_LIMIT_REGISTER_PER_MINUTE="${E2E_REGISTER_LIMIT:-1000}" \
"${TMP}/groupify-api" >"${TMP}/api.log" 2>&1 &
API_PID=$!

for _ in $(seq 1 60); do
  if curl -sf "${BASE_URL}/readyz" >/dev/null 2>&1; then break; fi
  if ! kill -0 "${API_PID}" 2>/dev/null; then
    echo "the API exited during startup"
    tail -30 "${TMP}/api.log" || true
    exit 1
  fi
  sleep 1
done
curl -sf "${BASE_URL}/readyz" >/dev/null || { echo "the API never became ready"; exit 1; }

echo "==> seeding the admin account"
DATABASE_URL="${DATABASE_URL}" \
ADMIN_EMAIL="${ADMIN_EMAIL}" \
ADMIN_PASSWORD="${ADMIN_PASSWORD}" \
"${TMP}/groupify-seed" >/dev/null

echo "==> running Playwright"
(cd "${ROOT}/frontend" &&
  E2E_BASE_URL="${BASE_URL}" \
  E2E_ADMIN_EMAIL="${ADMIN_EMAIL}" \
  E2E_ADMIN_PASSWORD="${ADMIN_PASSWORD}" \
  npx playwright test "$@")
