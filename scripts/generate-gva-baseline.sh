#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
env_file="${MOONBOOK_ENV_FILE:-$root_dir/.env}"
source_db="${GVA_BASELINE_DATABASE:-moonbook_gva_baseline_20260814}"
output="$root_dir/server/internal/platform/migrate/migrations/00002_gva_foundation.sql"
seed_output="$root_dir/server/internal/platform/migrate/migrations/00003_gva_seed.sql"

if [[ ! -f "$env_file" ]]; then
  echo "missing $env_file" >&2
  exit 2
fi
set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
schema="$tmp_dir/schema.sql"

docker compose --project-directory "$root_dir" --env-file "$env_file" -f "$root_dir/compose.yaml" \
  exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$source_db" \
  --schema-only --no-owner --no-privileges >"$schema"

{
  printf '%s\n' '-- +goose Up' '-- +goose StatementBegin'
  sed -E \
    -e '/^--$/d' \
    -e '/^-- PostgreSQL database dump/d' \
    -e '/^-- Dumped /d' \
    -e '/^-- Name:/d' \
    -e '/^\\(un)?restrict /d' \
    -e '/^SET (statement_timeout|lock_timeout|idle_in_transaction_session_timeout|transaction_timeout|client_encoding|standard_conforming_strings|check_function_bodies|xmloption|client_min_messages|row_security|default_tablespace|default_table_access_method)/d' \
    -e "/^SELECT pg_catalog.set_config\('search_path'/d" \
    "$schema" | awk 'NF { blank=0; print; next } !blank { print; blank=1 }'
  printf '%s\n' '-- +goose StatementEnd' '' '-- +goose Down' '-- Moonbook migrations are forward-only.' \
    '-- +goose StatementBegin' 'DO $$' 'BEGIN' \
    "    RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration';" \
    'END' '$$;' '-- +goose StatementEnd'
} >"$output"

# Only framework-wide, non-user seed tables are exported. Admin users, user-role
# links and sample upload records are deliberately excluded.
seed_tables=(
  sys_authorities sys_departments sys_positions
  sys_dictionaries sys_dictionary_details
  sys_base_menus sys_authority_menus
  sys_apis sys_ignore_apis casbin_rule
  sys_export_templates sys_security_config sys_timed_tasks
)
{
  printf '%s\n' '-- +goose Up' '-- +goose StatementBegin'
  for table in "${seed_tables[@]}"; do
    docker compose --project-directory "$root_dir" --env-file "$env_file" -f "$root_dir/compose.yaml" \
      exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$source_db" \
      --data-only --inserts --column-inserts --on-conflict-do-nothing \
      --no-owner --no-privileges --table="$table" |
      awk -v table="$table" '
        /^INSERT INTO / {
          if (in_insert) {
            print "nested INSERT while exporting " table > "/dev/stderr"
            exit 2
          }
          in_insert = 1
          skip_insert = (table == "sys_ignore_apis" && $0 ~ /\/init\/initdb/)
        }
        in_insert {
          if (!skip_insert) {
            print
          }
          if ($0 ~ /ON CONFLICT DO NOTHING;[[:space:]]*$/) {
            in_insert = 0
            skip_insert = 0
          }
          next
        }
        /^SELECT pg_catalog\.setval\(.*;[[:space:]]*$/ { print }
        END {
          if (in_insert) {
            print "unterminated INSERT while exporting " table > "/dev/stderr"
            exit 3
          }
        }
      '
  done
  printf '%s\n' '-- +goose StatementEnd' '' '-- +goose Down' '-- Moonbook migrations are forward-only.' \
    '-- +goose StatementBegin' 'DO $$' 'BEGIN' \
    "    RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration';" \
    'END' '$$;' '-- +goose StatementEnd'
} >"$seed_output"

if rg -n -i '\b(drop database|truncate|delete[[:space:]]+from)\b' "$output" "$seed_output"; then
  echo "generated migration contains forbidden destructive SQL" >&2
  exit 3
fi
if rg -n 'MoonbookBaselineOnly|moonbook_local_|password[[:space:]]*=' "$output" "$seed_output"; then
  echo "generated migration contains a credential marker" >&2
  exit 4
fi
if rg -n '/init/initdb' "$seed_output"; then
  echo "generated seed contains disabled HTTP database initialization metadata" >&2
  exit 5
fi
insert_count="$(rg -c '^INSERT INTO ' "$seed_output")"
complete_insert_count="$(rg -c 'ON CONFLICT DO NOTHING;$' "$seed_output")"
if [[ "$insert_count" != 771 || "$complete_insert_count" != 771 ]]; then
  echo "generated seed contains $insert_count INSERT statements, $complete_insert_count complete; expected 771" >&2
  exit 6
fi
printf 'generated %s and %s\n' "$output" "$seed_output"
