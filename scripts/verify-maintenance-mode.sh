#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
project="${MOONBOOK_VERIFY_PROJECT:-moonbook_verify_maintenance}"
env_file="$(mktemp "${TMPDIR:-/tmp}/moonbook-maintenance-env.XXXXXX")"

pick_port() {
  node -e 'const s=require("node:net").createServer();s.listen(0,"127.0.0.1",()=>{process.stdout.write(String(s.address().port));s.close()})'
}
postgres_port="${MOONBOOK_VERIFY_POSTGRES_PORT:-$(pick_port)}"
redis_port="${MOONBOOK_VERIFY_REDIS_PORT:-$(pick_port)}"
minio_port="${MOONBOOK_VERIFY_MINIO_PORT:-$(pick_port)}"
minio_console_port="${MOONBOOK_VERIFY_MINIO_CONSOLE_PORT:-$(pick_port)}"
admin_port="${MOONBOOK_VERIFY_ADMIN_PORT:-$(pick_port)}"
reader_port="${MOONBOOK_VERIFY_READER_PORT:-$(pick_port)}"

case "$project" in moonbook_verify_*) ;; *)
  echo "MOONBOOK_VERIFY_PROJECT must start with moonbook_verify_: $project" >&2
  exit 2
esac

cleanup() {
  local status=$?
  if [[ "${MOONBOOK_VERIFY_KEEP:-0}" != 1 ]]; then
    compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  fi
  rm -f "$env_file"
  exit "$status"
}
trap cleanup EXIT INT TERM

cat >"$env_file" <<EOF
COMPOSE_PROJECT_NAME=$project
POSTGRES_IMAGE=postgres:17.6-alpine
POSTGRES_HOST_PORT=$postgres_port
POSTGRES_DB=moonbook
POSTGRES_USER=moonbook
POSTGRES_PASSWORD=maintenance-verify-postgres
REDIS_IMAGE=redis:7.4.5-alpine
REDIS_HOST_PORT=$redis_port
REDIS_PASSWORD=maintenance-verify-redis
MINIO_IMAGE=minio/minio:RELEASE.2025-07-23T15-54-02Z
MINIO_MC_IMAGE=minio/mc:RELEASE.2025-07-21T05-28-08Z
MINIO_API_HOST_PORT=$minio_port
MINIO_CONSOLE_HOST_PORT=$minio_console_port
MINIO_ROOT_USER=moonbook
MINIO_ROOT_PASSWORD=maintenance-verify-minio
MINIO_BUCKET=moonbook-content
MOONBOOK_JWT_SIGNING_KEY=maintenance-verify-jwt-signing-key
MOONBOOK_METRICS_TOKEN=maintenance-verify-metrics-token
MOONBOOK_ADMIN_HOST_PORT=$admin_port
MOONBOOK_READER_HOST_PORT=$reader_port
MOONBOOK_ADMIN_ORIGIN=http://localhost:$admin_port
MOONBOOK_READER_ORIGIN=http://localhost:$reader_port
MOONBOOK_TRUSTED_PROXIES=172.16.0.0/12
VITE_READER_API_BASE=/prod-api
EOF
chmod 600 "$env_file"

compose() {
  docker compose --project-directory "$root_dir" --env-file "$env_file" -f "$root_dir/compose.yaml" "$@"
}

echo "[1/5] validating and starting isolated stack"
compose config --quiet
compose up -d --build

echo "[2/5] entering maintenance mode"
"$root_dir/scripts/maintenance-mode.sh" on --env-file "$env_file" --confirm-project "$project"
test "$("$root_dir/scripts/maintenance-mode.sh" status --env-file "$env_file")" = maintenance

echo "[3/5] verifying data services remain healthy"
compose exec -T postgres pg_isready -U moonbook -d moonbook >/dev/null
test "$(compose exec -T redis redis-cli -a maintenance-verify-redis --no-auth-warning ping | tr -d '\r')" = PONG
compose exec -T minio curl -fsS http://127.0.0.1:9000/minio/health/ready >/dev/null

echo "[4/5] leaving maintenance mode"
"$root_dir/scripts/maintenance-mode.sh" off --env-file "$env_file" --confirm-project "$project"
test "$("$root_dir/scripts/maintenance-mode.sh" status --env-file "$env_file")" = active

echo "[5/5] verifying normal API and Reader routes"
curl -fsS "http://127.0.0.1:$admin_port/api/health/ready" | grep -q '"status":"ok"'
test "$(curl -fsS "http://127.0.0.1:$reader_port/health")" = ok
echo "maintenance mode verification passed for isolated project $project"
