#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
env_file="${MOONBOOK_ENV_FILE:-$root_dir/.env}"
template="$root_dir/server/config.moonbook.yaml.tpl"
output="$root_dir/server/config.moonbook.local.yaml"

if [[ ! -f "$env_file" ]]; then
  echo "missing $env_file; copy .env.example to .env first" >&2
  exit 2
fi

set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

required=(
  POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD
  REDIS_PASSWORD
  MINIO_ROOT_USER MINIO_ROOT_PASSWORD MINIO_BUCKET
  MOONBOOK_SERVER_PORT MOONBOOK_JWT_SIGNING_KEY MOONBOOK_METRICS_TOKEN
)

for name in "${required[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    echo "missing required value in $env_file: $name" >&2
    exit 3
  fi
done

MOONBOOK_POSTGRES_HOST="${MOONBOOK_POSTGRES_HOST:-127.0.0.1}"
MOONBOOK_POSTGRES_PORT="${MOONBOOK_POSTGRES_PORT:-${POSTGRES_HOST_PORT:-15432}}"
MOONBOOK_REDIS_ADDR="${MOONBOOK_REDIS_ADDR:-127.0.0.1:${REDIS_HOST_PORT:-16379}}"
MOONBOOK_MINIO_ENDPOINT="${MOONBOOK_MINIO_ENDPOINT:-127.0.0.1:${MINIO_API_HOST_PORT:-19000}}"
MOONBOOK_MINIO_BUCKET_URL="${MOONBOOK_MINIO_BUCKET_URL:-http://${MOONBOOK_MINIO_ENDPOINT}/${MINIO_BUCKET}}"
export MOONBOOK_POSTGRES_HOST MOONBOOK_POSTGRES_PORT MOONBOOK_REDIS_ADDR
export MOONBOOK_MINIO_ENDPOINT MOONBOOK_MINIO_BUCKET_URL

cd "$root_dir/server"
CGO_ENABLED=0 go run ./cmd/moonbook-config render "$template" "$output"
printf 'rendered %s\n' "$output"
