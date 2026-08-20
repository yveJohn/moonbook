#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
env_file="${1:-${MOONBOOK_ENV_FILE:-$root_dir/.env}}"
[[ "$env_file" == /* ]] || env_file="$root_dir/$env_file"
[[ -f "$env_file" ]] || { echo "production env file not found: $env_file" >&2; exit 2; }

for command in docker node; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 2; }
done

docker compose \
  --project-directory "$root_dir" \
  --env-file "$env_file" \
  -f "$root_dir/compose.yaml" \
  -f "$root_dir/deploy/compose/compose.production.yaml" \
  config --format json | node "$root_dir/scripts/verify-production-config.mjs"
