#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  MYSQL_HOST=127.0.0.1 MYSQL_PORT=3306 MYSQL_USER=readonly \
  MYSQL_PASSWORD='...' MYSQL_DATABASE=moonbook_admin \
  scripts/inventory-legacy-mysql.sh [output.tsv]

The account must be read-only. The script only queries information_schema
metadata estimates; it never changes the source database.
USAGE
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

for name in MYSQL_HOST MYSQL_PORT MYSQL_USER MYSQL_PASSWORD MYSQL_DATABASE; do
  if [[ -z "${!name:-}" ]]; then
    echo "missing required environment variable: ${name}" >&2
    usage >&2
    exit 2
  fi
done

if ! command -v mysql >/dev/null 2>&1; then
  echo "mysql client is required" >&2
  exit 3
fi

output="${1:-legacy-mysql-inventory.tsv}"
temp_file="$(mktemp)"
trap 'rm -f "$temp_file"' EXIT

mysql_args=(
  --batch
  --raw
  --skip-column-names
  --host="$MYSQL_HOST"
  --port="$MYSQL_PORT"
  --user="$MYSQL_USER"
  --database="$MYSQL_DATABASE"
)

MYSQL_PWD="$MYSQL_PASSWORD" mysql "${mysql_args[@]}" <<'SQL' >"$temp_file"
SELECT 'database', DATABASE(), '', '', '';
SELECT 'server_version', VERSION(), '', '', '';
SELECT
  'table',
  table_name,
  table_rows,
  data_length,
  index_length
FROM information_schema.tables
WHERE table_schema = DATABASE()
  AND table_type = 'BASE TABLE'
ORDER BY table_name;
SQL

{
  printf 'record_type\tname\testimated_rows\tdata_bytes\tindex_bytes\n'
  cat "$temp_file"
} >"$output"

printf 'wrote read-only inventory to %s\n' "$output"
