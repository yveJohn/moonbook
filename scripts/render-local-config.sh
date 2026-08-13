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
  POSTGRES_HOST_PORT POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD
  REDIS_HOST_PORT REDIS_PASSWORD
  MINIO_API_HOST_PORT MINIO_ROOT_USER MINIO_ROOT_PASSWORD MINIO_BUCKET
  MOONBOOK_SERVER_PORT MOONBOOK_JWT_SIGNING_KEY
)

for name in "${required[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    echo "missing required value in $env_file: $name" >&2
    exit 3
  fi
done

variables='${POSTGRES_HOST_PORT} ${POSTGRES_DB} ${POSTGRES_USER} ${POSTGRES_PASSWORD} ${REDIS_HOST_PORT} ${REDIS_PASSWORD} ${MINIO_API_HOST_PORT} ${MINIO_ROOT_USER} ${MINIO_ROOT_PASSWORD} ${MINIO_BUCKET} ${MOONBOOK_SERVER_PORT} ${MOONBOOK_JWT_SIGNING_KEY}'
envsubst "$variables" <"$template" >"$output"
chmod 600 "$output"
printf 'rendered %s\n' "$output"
