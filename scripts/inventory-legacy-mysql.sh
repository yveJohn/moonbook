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

required_names=(MYSQL_USER MYSQL_PASSWORD MYSQL_DATABASE)
if [[ -z "${MYSQL_DOCKER_CONTAINER:-}" ]]; then
  required_names+=(MYSQL_HOST MYSQL_PORT)
fi
for name in "${required_names[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    echo "missing required environment variable: ${name}" >&2
    usage >&2
    exit 2
  fi
done

if [[ -n "${MYSQL_DOCKER_CONTAINER:-}" ]]; then
  command -v docker >/dev/null 2>&1 || { echo "docker is required for MYSQL_DOCKER_CONTAINER" >&2; exit 3; }
else
  command -v mysql >/dev/null 2>&1 || { echo "mysql client is required" >&2; exit 3; }
fi

output="${1:-legacy-mysql-inventory.tsv}"
temp_file="$(mktemp)"
trap 'rm -f "$temp_file"' EXIT

if [[ -n "${MYSQL_DOCKER_CONTAINER:-}" ]]; then
  mysql_command=(docker exec -i -e MYSQL_PWD="$MYSQL_PASSWORD" "$MYSQL_DOCKER_CONTAINER" mysql --batch --raw --skip-column-names --user="$MYSQL_USER" --database="$MYSQL_DATABASE")
else
  mysql_args=(
    --batch
    --raw
    --skip-column-names
    --host="$MYSQL_HOST"
    --port="$MYSQL_PORT"
    --user="$MYSQL_USER"
    --database="$MYSQL_DATABASE"
  )
  mysql_command=(env MYSQL_PWD="$MYSQL_PASSWORD" mysql "${mysql_args[@]}")
fi

"${mysql_command[@]}" <<'SQL' >"$temp_file"
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
