#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
env_file="${MOONBOOK_ENV_FILE:-$root_dir/.env}"

if [[ ! -f "$env_file" ]]; then
  echo "missing $env_file; copy .env.example to .env first" >&2
  exit 2
fi

set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

compose=(docker compose --project-directory "$root_dir" --env-file "$env_file" -f "$root_dir/compose.yaml")

"${compose[@]}" exec -T postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"
"${compose[@]}" exec -T redis redis-cli -a "$REDIS_PASSWORD" --no-auth-warning ping | grep -qx PONG
"${compose[@]}" exec -T minio curl -fsS http://localhost:9000/minio/health/ready >/dev/null
"${compose[@]}" run --rm --no-deps minio-init >/dev/null
bucket_policy="$(${compose[@]} run --rm --no-deps --entrypoint /bin/sh minio-init -ec '
  mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
  mc anonymous get "local/$MINIO_BUCKET"
')"
grep -q 'is `private`' <<<"$bucket_policy"

printf 'PostgreSQL, Redis and MinIO are healthy; bucket %s exists and is private.\n' "$MINIO_BUCKET"
