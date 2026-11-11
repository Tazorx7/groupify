#!/usr/bin/env bash
# Smoke test of the public + admin API surface against a running instance.
# Usage: BASE=http://127.0.0.1:8081 ADMIN_EMAIL=... ADMIN_PASSWORD=... ./scripts/smoke.sh
set -uo pipefail

BASE="${BASE:-http://127.0.0.1:8081}"
EMAIL="${ADMIN_EMAIL:-rep@example.edu}"
PASSWORD="${ADMIN_PASSWORD:-change-me-please-123}"
DATABASE_URL="${DATABASE_URL:-postgres://groupify:groupify@127.0.0.1:5433/groupify?sslmode=disable}"

# --reset wipes the database first. The assertions below describe a whole
# lifecycle, so they need a known starting point.
if [ "${1:-}" = "--reset" ]; then
  echo "resetting database ${DATABASE_URL%%\?*}"
  psql "$DATABASE_URL" -q -c "
    TRUNCATE registrations, topics, audit_logs, idempotency_keys,
             admin_sessions, admins, app_settings RESTART IDENTITY CASCADE;
    INSERT INTO app_settings (id, registration_open) VALUES (1, false);" || exit 1
  ( cd "$(dirname "$0")/.." && make seed >/dev/null ) || exit 1
fi

JAR=$(mktemp)
BODY=$(mktemp)
trap 'rm -f "$JAR" "$BODY"' EXIT

pass=0; fail=0
check() { # name expected actual
  if [ "$2" = "$3" ]; then printf '  ok   %-52s %s\n' "$1" "$3"; pass=$((pass+1));
  else printf '  FAIL %-52s expected=%s actual=%s\n' "$1" "$2" "$3"; fail=$((fail+1)); fi
}
code() { curl -s -o "$BODY" -w '%{http_code}' "$@"; }
json() { python3 -c "import json,sys;d=json.load(open('$BODY'));print(eval(sys.argv[1]))" "$1" 2>/dev/null; }

echo "== health =="
check "GET /healthz" 200 "$(code "$BASE/healthz")"
check "GET /readyz"  200 "$(code "$BASE/readyz")"

echo "== unauthenticated =="
check "GET /api/admin/overview is 401" 401 "$(code "$BASE/api/admin/overview")"
check "GET /api/admin/registrations is 401" 401 "$(code "$BASE/api/admin/registrations")"

echo "== login =="
check "POST /api/admin/login" 200 "$(code -c "$JAR" -X POST "$BASE/api/admin/login" \
  -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")"
CSRF=$(json "d['csrf_token']")

check "login with wrong password is 401" 401 "$(code -X POST "$BASE/api/admin/login" \
  -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\",\"password\":\"definitely-wrong\"}")"

echo "== csrf =="
check "POST topic without CSRF header is 403" 403 "$(code -b "$JAR" -X POST "$BASE/api/admin/topics" \
  -H 'Content-Type: application/json' -d '{"name":"No CSRF","capacity":5}')"

echo "== topics =="
check "create topic A" 201 "$(code -b "$JAR" -X POST "$BASE/api/admin/topics" \
  -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" \
  -d '{"name":"Smoke A","description":"first","capacity":2}')"
TOPIC_A=$(json "d['id']")
check "create topic B" 201 "$(code -b "$JAR" -X POST "$BASE/api/admin/topics" \
  -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" \
  -d '{"name":"Smoke B","description":"second","capacity":10}')"
TOPIC_B=$(json "d['id']")
check "duplicate topic name is 409" 409 "$(code -b "$JAR" -X POST "$BASE/api/admin/topics" \
  -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" \
  -d '{"name":"smoke a","capacity":3}')"

echo "== registration closed =="
check "register while closed is 409" 409 "$(code -X POST "$BASE/api/registrations" \
  -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"Ama Mensah\",\"index_number\":\"00123456\",\"topic_id\":$TOPIC_B}")"

echo "== open registration =="
check "open registration" 200 "$(code -b "$JAR" -X PATCH "$BASE/api/admin/settings" \
  -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" \
  -d '{"registration_open":true}')"

echo "== registration =="
check "register student 1 (normalised index)" 201 "$(code -X POST "$BASE/api/registrations" \
  -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"  Ama   Mensah \",\"index_number\":\" cs/2024/045 \",\"topic_id\":$TOPIC_B}")"
check "index normalised to CS/2024/045" "CS/2024/045" "$(json "d['index_number']")"
check "name whitespace collapsed" "Ama Mensah" "$(json "d['full_name']")"

check "same index lower-case is 409" 409 "$(code -X POST "$BASE/api/registrations" \
  -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"Someone Else\",\"index_number\":\"CS/2024/045\",\"topic_id\":$TOPIC_A}")"

check "unknown topic is 404" 404 "$(code -X POST "$BASE/api/registrations" \
  -H 'Content-Type: application/json' \
  -d '{"full_name":"Kofi Boateng","index_number":"777","topic_id":999999}')"

check "blank name is 422" 422 "$(code -X POST "$BASE/api/registrations" \
  -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"   \",\"index_number\":\"888\",\"topic_id\":$TOPIC_A}")"

echo "== capacity =="
check "fill topic A slot 1" 201 "$(code -X POST "$BASE/api/registrations" -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"Kofi Boateng\",\"index_number\":\"A-100\",\"topic_id\":$TOPIC_A}")"
check "fill topic A slot 2" 201 "$(code -X POST "$BASE/api/registrations" -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"Yaa Asantewaa\",\"index_number\":\"A-101\",\"topic_id\":$TOPIC_A}")"
check "topic A is now full (409)" 409 "$(code -X POST "$BASE/api/registrations" -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"Kwame Nkrumah\",\"index_number\":\"A-102\",\"topic_id\":$TOPIC_A}")"

echo "== idempotency =="
KEY="smoke-key-$(date +%s)-aaaa"
check "first idempotent request" 201 "$(code -X POST "$BASE/api/registrations" \
  -H 'Content-Type: application/json' -H "Idempotency-Key: $KEY" \
  -d "{\"full_name\":\"Efua Owusu\",\"index_number\":\"B-200\",\"topic_id\":$TOPIC_B}")"
FIRST_ID=$(json "d['registration_id']")
check "replay returns 201" 201 "$(code -X POST "$BASE/api/registrations" \
  -H 'Content-Type: application/json' -H "Idempotency-Key: $KEY" \
  -d "{\"full_name\":\"Efua Owusu\",\"index_number\":\"B-200\",\"topic_id\":$TOPIC_B}")"
check "replay returns same registration" "$FIRST_ID" "$(json "d['registration_id']")"
check "key reuse with different body is 422" 422 "$(code -X POST "$BASE/api/registrations" \
  -H 'Content-Type: application/json' -H "Idempotency-Key: $KEY" \
  -d "{\"full_name\":\"Other Person\",\"index_number\":\"B-201\",\"topic_id\":$TOPIC_B}")"

echo "== admin reads =="
check "GET overview" 200 "$(code -b "$JAR" "$BASE/api/admin/overview")"
echo "     $(cat "$BODY")"
check "GET topics" 200 "$(code -b "$JAR" "$BASE/api/admin/topics")"
check "GET registrations" 200 "$(code -b "$JAR" "$BASE/api/admin/registrations?limit=100")"
echo "     total=$(json "d['total']")"
check "search by name" 200 "$(code -b "$JAR" "$BASE/api/admin/registrations?search=ama")"
echo "     matches=$(json "d['total']")"
check "search by index" 200 "$(code -b "$JAR" "$BASE/api/admin/registrations?search=cs/2024")"
echo "     matches=$(json "d['total']")"
check "filter by topic" 200 "$(code -b "$JAR" "$BASE/api/admin/registrations?topic_id=$TOPIC_A")"
echo "     matches=$(json "d['total']")"
EXPORT=$(mktemp)
check "export csv" 200 "$(curl -s -b "$JAR" -o "$EXPORT" -w '%{http_code}' "$BASE/api/admin/registrations/export")"
echo "     header: $(head -1 "$EXPORT" | tr -d '\357\273\277')"
echo "     row1:   $(sed -n '2p' "$EXPORT")"
check "csv has one row per registration" "4" "$(($(wc -l < "$EXPORT") - 1))"
rm -f "$EXPORT"

echo "== capacity guard =="
check "reduce capacity below occupancy is 409" 409 "$(code -b "$JAR" -X PATCH "$BASE/api/admin/topics/$TOPIC_A" \
  -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" -d '{"capacity":1}')"
check "raise capacity to 3" 200 "$(code -b "$JAR" -X PATCH "$BASE/api/admin/topics/$TOPIC_A" \
  -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" -d '{"capacity":3}')"
check "new slot accepts a student" 201 "$(code -X POST "$BASE/api/registrations" -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"Theophilus Tagoe\",\"index_number\":\"A-103\",\"topic_id\":$TOPIC_A}")"
NEW_ID=$(json "d['registration_id']")

echo "== deactivate =="
check "deactivate topic A" 200 "$(code -b "$JAR" -X POST "$BASE/api/admin/topics/$TOPIC_A/deactivate" \
  -H "X-CSRF-Token: $CSRF")"
check "inactive topic rejects registration" 409 "$(code -X POST "$BASE/api/registrations" -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"Another Person\",\"index_number\":\"A-104\",\"topic_id\":$TOPIC_A}")"

echo "== admin removal =="
check "delete registration" 204 "$(code -b "$JAR" -X DELETE "$BASE/api/admin/registrations/$NEW_ID" \
  -H "X-CSRF-Token: $CSRF")"
check "deleted registration is gone (404)" 404 "$(code -b "$JAR" -X DELETE "$BASE/api/admin/registrations/$NEW_ID" \
  -H "X-CSRF-Token: $CSRF")"
check "reactivate topic A" 200 "$(code -b "$JAR" -X POST "$BASE/api/admin/topics/$TOPIC_A/activate" \
  -H "X-CSRF-Token: $CSRF")"
check "freed slot accepts a student" 201 "$(code -X POST "$BASE/api/registrations" -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"Abena Pokuaa\",\"index_number\":\"A-105\",\"topic_id\":$TOPIC_A}")"

echo "== audit =="
check "GET audit logs" 200 "$(code -b "$JAR" "$BASE/api/admin/audit-logs?limit=100")"
echo "     actions: $(json "' '.join(sorted({e['action'] for e in d['items']}))")"
check "audit contains no index numbers" "True" "$(python3 -c "
import json
d=json.load(open('$BODY'))
blob=json.dumps(d)
print(not any(x in blob for x in ['A-100','A-101','CS/2024/045','00123456','B-200']))
")"

echo "== close registration =="
check "close registration" 200 "$(code -b "$JAR" -X PATCH "$BASE/api/admin/settings" \
  -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" -d '{"registration_open":false}')"
check "register after close is 409" 409 "$(code -X POST "$BASE/api/registrations" -H 'Content-Type: application/json' \
  -d "{\"full_name\":\"Late Student\",\"index_number\":\"Z-999\",\"topic_id\":$TOPIC_B}")"

echo "== logout =="
check "logout" 204 "$(code -b "$JAR" -X POST "$BASE/api/admin/logout" -H "X-CSRF-Token: $CSRF")"
check "session revoked (401)" 401 "$(code -b "$JAR" "$BASE/api/admin/overview")"

echo
echo "passed=$pass failed=$fail"
[ "$fail" -eq 0 ]
