#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
project="${MOONBOOK_VERIFY_PROJECT:-moonbook_verify_m1}"
env_file="$(mktemp "${TMPDIR:-/tmp}/moonbook-m1-env.XXXXXX")"
response_file="$(mktemp "${TMPDIR:-/tmp}/moonbook-m1-response.XXXXXX")"
header_file="$(mktemp "${TMPDIR:-/tmp}/moonbook-m1-headers.XXXXXX")"

case "$project" in
  moonbook_verify_*) ;;
  *) echo "MOONBOOK_VERIFY_PROJECT must start with moonbook_verify_: $project" >&2; exit 2 ;;
esac

admin_port="${MOONBOOK_VERIFY_ADMIN_PORT:-28080}"
reader_port="${MOONBOOK_VERIFY_READER_PORT:-28081}"
postgres_port="${MOONBOOK_VERIFY_POSTGRES_PORT:-25432}"
redis_port="${MOONBOOK_VERIFY_REDIS_PORT:-26379}"
minio_port="${MOONBOOK_VERIFY_MINIO_PORT:-29000}"
minio_console_port="${MOONBOOK_VERIFY_MINIO_CONSOLE_PORT:-29001}"
admin_username="m1verify_admin"
admin_password="M1Verify-Local-Only-2026"
metrics_token="m1-verify-local-metrics-token"

cleanup() {
  local status=$?
  if [[ "${MOONBOOK_VERIFY_KEEP:-0}" != "1" ]]; then
    compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  fi
  rm -f "$env_file" "$response_file" "$header_file"
  exit "$status"
}
trap cleanup EXIT INT TERM

cat >"$env_file" <<EOF
COMPOSE_PROJECT_NAME=$project
POSTGRES_IMAGE=postgres:17.6-alpine
POSTGRES_HOST_PORT=$postgres_port
POSTGRES_DB=moonbook
POSTGRES_USER=moonbook
POSTGRES_PASSWORD=m1-verify-local-postgres
REDIS_IMAGE=redis:7.4.5-alpine
REDIS_HOST_PORT=$redis_port
REDIS_PASSWORD=m1-verify-local-redis
MINIO_IMAGE=minio/minio:RELEASE.2025-07-23T15-54-02Z
MINIO_MC_IMAGE=minio/mc:RELEASE.2025-07-21T05-28-08Z
MINIO_API_HOST_PORT=$minio_port
MINIO_CONSOLE_HOST_PORT=$minio_console_port
MINIO_ROOT_USER=moonbook
MINIO_ROOT_PASSWORD=m1-verify-local-minio
MINIO_BUCKET=moonbook-content
MOONBOOK_SERVER_PORT=18888
MOONBOOK_JWT_SIGNING_KEY=m1-verify-local-jwt-signing-key
MOONBOOK_METRICS_TOKEN=$metrics_token
MOONBOOK_ADMIN_HOST_PORT=$admin_port
MOONBOOK_READER_HOST_PORT=$reader_port
MOONBOOK_READER_ORIGIN=http://localhost:$reader_port
VITE_READER_API_BASE=/prod-api
MOONBOOK_ADMIN_USERNAME=$admin_username
MOONBOOK_ADMIN_PASSWORD=$admin_password
MOONBOOK_ADMIN_NICKNAME=M1 Verify Admin
EOF
chmod 600 "$env_file"

compose() {
  docker compose --project-directory "$root_dir" --env-file "$env_file" -f "$root_dir/compose.yaml" "$@"
}

wait_http() {
  local url="$1" i
  for i in $(seq 1 60); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url" >&2
  return 1
}

echo "[1/9] validating Compose model"
compose config --quiet

echo "[2/9] building application images"
compose build migrate web reader-ui

echo "[3/9] starting an empty isolated stack"
compose up -d
wait_http "http://127.0.0.1:$admin_port/gateway-health"

echo "[4/9] verifying infrastructure and migrations"
echo "  - PostgreSQL readiness"
compose exec -T postgres pg_isready -U moonbook -d moonbook >/dev/null || {
  echo "PostgreSQL readiness failed" >&2
  exit 1
}
echo "  - Redis authenticated ping"
redis_ping="$(compose exec -T redis redis-cli -a m1-verify-local-redis --no-auth-warning ping | tr -d '\r')"
test "$redis_ping" = "PONG" || {
  echo "Redis authenticated ping failed" >&2
  exit 1
}
echo "  - MinIO readiness"
compose exec -T minio curl -fsS http://127.0.0.1:9000/minio/health/ready >/dev/null || {
  echo "MinIO readiness failed" >&2
  exit 1
}
echo "  - migration status"
compose run --rm migrate moonbook-migrate status >"$response_file"
status_line="$(grep -E '^current=[0-9]+ target=[0-9]+ pending=(true|false)$' "$response_file" || true)"
current_version="$(sed -n 's/^current=\([0-9][0-9]*\) .*/\1/p' <<<"$status_line")"
target_version="$(sed -n 's/.* target=\([0-9][0-9]*\) .*/\1/p' <<<"$status_line")"
if [[ -z "$current_version" || "$current_version" != "$target_version" || "$status_line" != *"pending=false" ]]; then
  echo "migration status is not current" >&2
  exit 1
fi
echo "  - migration idempotency"
compose run --rm migrate moonbook-migrate up >"$response_file"
grep -q 'applied=0' "$response_file" || {
  echo "migration rerun was not idempotent" >&2
  exit 1
}

echo "[5/9] verifying runtime isolation and routes"
compose exec -T server id | grep -q 'uid=100(moonbook)'
curl -fsS -o "$response_file" "http://127.0.0.1:$admin_port/"
grep -qi '<html' "$response_file"
test "$(curl -fsS "http://127.0.0.1:$reader_port/health")" = "ok"
curl -fsS -o "$response_file" "http://127.0.0.1:$reader_port/prod-api/health/live"
grep -q '"status":"ok"' "$response_file"
curl -fsS -o "$response_file" "http://127.0.0.1:$reader_port/dev-api/health/live"
grep -q '"status":"ok"' "$response_file"
status="$(curl -sS -o "$response_file" -D "$header_file" -w '%{http_code}' \
  -H "Origin: http://localhost:$reader_port" \
  "http://127.0.0.1:$reader_port/prod-api/health/live")"
test "$status" = "200"
grep -qi "^Access-Control-Allow-Origin: http://localhost:$reader_port" "$header_file"
status="$(curl -sS -o "$response_file" -w '%{http_code}' \
  -H 'Origin: https://untrusted.example' \
  "http://127.0.0.1:$reader_port/prod-api/health/live")"
test "$status" = "403"
curl -fsS -o "$response_file" "http://127.0.0.1:$admin_port/api/health/live"
grep -q '"status":"ok"' "$response_file"
curl -fsS -o "$response_file" "http://127.0.0.1:$admin_port/api/health/ready"
grep -q '"status":"ok"' "$response_file"

echo "[6/9] verifying metrics authentication"
status="$(curl -sS -o "$response_file" -w '%{http_code}' "http://127.0.0.1:$admin_port/api/metrics")"
test "$status" = "401"
curl -fsS -o "$response_file" -H "Authorization: Bearer $metrics_token" "http://127.0.0.1:$admin_port/api/metrics"
grep -q '^moonbook_http_requests_total' "$response_file"

echo "[7/9] bootstrapping and logging in as the first administrator"
compose --profile bootstrap run --rm admin-bootstrap | grep -q 'administrator_created=true'
compose exec -T postgres psql -U moonbook -d moonbook -v ON_ERROR_STOP=1 -c \
  "update sys_security_config set captcha_open=999 where id=1" >/dev/null
compose restart server >/dev/null
wait_http "http://127.0.0.1:$admin_port/api/health/ready"
status="$(curl -sS -o "$response_file" -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  --data "{\"username\":\"$admin_username\",\"password\":\"$admin_password\",\"captcha\":\"\",\"captchaId\":\"\"}" \
  "http://127.0.0.1:$admin_port/api/base/login")"
test "$status" = "200"
grep -q '"code":0' "$response_file"
grep -q '"needChangePassword":true' "$response_file"
compose exec -T postgres psql -U moonbook -d moonbook -v ON_ERROR_STOP=1 -c \
  "update sys_security_config set captcha_open=0 where id=1" >/dev/null
compose restart server >/dev/null
wait_http "http://127.0.0.1:$admin_port/api/health/ready"
test "$(compose exec -T postgres psql -U moonbook -d moonbook -Atc 'select captcha_open from sys_security_config where id=1')" = "0"

echo "[8/9] verifying graceful API shutdown"
before="$(compose logs --no-color server | wc -l | tr -d ' ')"
compose stop --timeout 15 server >/dev/null
compose logs --no-color server | tail -n "+$((before + 1))" >"$response_file"
grep -q '关闭WEB服务' "$response_file"
grep -q 'WEB服务已关闭' "$response_file"
compose up -d server reader-ui gateway >/dev/null
wait_http "http://127.0.0.1:$admin_port/api/health/ready"

echo "[9/9] verifying repository secrets"
"$root_dir/scripts/scan-secrets.sh"

echo "M1 verification passed: empty migrations, full Compose stack, routes, metrics, login, shutdown, and secret scan."
