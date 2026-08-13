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

required=(
  POSTGRES_HOST_PORT POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD
  MOONBOOK_ADMIN_USERNAME MOONBOOK_ADMIN_PASSWORD
)
for name in "${required[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    echo "missing required environment variable: $name" >&2
    exit 3
  fi
done

urlencode() {
  local value="$1" encoded="" char hex i
  for ((i = 0; i < ${#value}; i++)); do
    char="${value:i:1}"
    case "$char" in
      [a-zA-Z0-9.~_-]) encoded+="$char" ;;
      *) printf -v hex '%%%02X' "'$char"; encoded+="$hex" ;;
    esac
  done
  printf '%s' "$encoded"
}

dsn="postgres://$(urlencode "$POSTGRES_USER"):$(urlencode "$POSTGRES_PASSWORD")@127.0.0.1:${POSTGRES_HOST_PORT}/$(urlencode "$POSTGRES_DB")?sslmode=disable"
cd "$root_dir/server"
MOONBOOK_DATABASE_DSN="$dsn" CGO_ENABLED=0 go run ./cmd/moonbook-admin bootstrap
