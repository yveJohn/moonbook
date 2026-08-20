#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
# shellcheck disable=SC1091
source "$root_dir/scripts/lib/full-copy-common.sh"

usage() {
  cat <<'USAGE'
Usage: scripts/verify-m6-full-copy.sh [--preflight|--run|--cleanup]

Required environment:
  MOONBOOK_FULL_COPY_BACKUP          absolute .sql.gz path
  MOONBOOK_FULL_COPY_SHA256          expected lowercase SHA-256
  MOONBOOK_FULL_COPY_PROJECT         moonbook_verify_m6_* project name
  MOONBOOK_FULL_COPY_ENV_FILE        absolute rehearsal environment file
  MOONBOOK_FULL_COPY_WORK_DIR        absolute directory outside both repositories
  MOONBOOK_FULL_COPY_TXT_MANIFEST    absolute legacy TXT manifest when source tasks exist
  MOONBOOK_FULL_COPY_TXT_ROOT        absolute read-only TXT root when source tasks exist
  MOONBOOK_FULL_COPY_CONFIRM         M6_FULL_COPY_LOCAL_ISOLATED

--preflight is the default and does not start containers.
--run creates isolated local containers and volumes.
--cleanup deletes only the exact isolated Compose project after confirmation.
USAGE
}

action="${1:---preflight}"
case "$action" in
  --preflight|--run|--cleanup) ;;
  --help|-h) usage; exit 0 ;;
  *) usage >&2; full_copy_die "unknown action: $action" ;;
esac

for name in MOONBOOK_FULL_COPY_BACKUP MOONBOOK_FULL_COPY_SHA256 MOONBOOK_FULL_COPY_PROJECT MOONBOOK_FULL_COPY_ENV_FILE MOONBOOK_FULL_COPY_WORK_DIR MOONBOOK_FULL_COPY_CONFIRM; do
  [[ -n "${!name:-}" ]] || full_copy_die "missing required environment variable: $name"
done
[[ "$MOONBOOK_FULL_COPY_CONFIRM" == "M6_FULL_COPY_LOCAL_ISOLATED" ]] || full_copy_die "confirmation marker is invalid"
full_copy_validate_project "$MOONBOOK_FULL_COPY_PROJECT"
full_copy_require_absolute_file MOONBOOK_FULL_COPY_BACKUP "$MOONBOOK_FULL_COPY_BACKUP"
full_copy_require_absolute_file MOONBOOK_FULL_COPY_ENV_FILE "$MOONBOOK_FULL_COPY_ENV_FILE"
[[ "$MOONBOOK_FULL_COPY_SHA256" =~ ^[0-9a-f]{64}$ ]] || full_copy_die "expected SHA-256 must be 64 lowercase hex characters"

backup_path="$(full_copy_canonical_path "$MOONBOOK_FULL_COPY_BACKUP")"
work_dir="$(full_copy_canonical_path "$MOONBOOK_FULL_COPY_WORK_DIR")"
legacy_root="/Users/yve/Documents/moonbook"
if full_copy_path_within "$work_dir" "$root_dir" || full_copy_path_within "$work_dir" "$legacy_root"; then
  full_copy_die "work directory must be outside the active and frozen repositories"
fi
[[ "$work_dir" != "/" && "$work_dir" != "/Users" && "$work_dir" != "/Users/yve" ]] || full_copy_die "work directory is too broad"

full_copy_require_command docker
full_copy_require_command gzip
full_copy_require_command shasum
full_copy_require_command awk
docker compose version >/dev/null

actual_sha="$(full_copy_sha256 "$backup_path")"
[[ "$actual_sha" == "$MOONBOOK_FULL_COPY_SHA256" ]] || full_copy_die "backup SHA-256 does not match the approved value"
gzip -t "$backup_path"

compose_base=(docker compose --project-directory "$root_dir" --env-file "$MOONBOOK_FULL_COPY_ENV_FILE" -p "$MOONBOOK_FULL_COPY_PROJECT" -f "$root_dir/compose.yaml")

if [[ "$action" == "--cleanup" ]]; then
  [[ "${MOONBOOK_FULL_COPY_CLEANUP_CONFIRM:-}" == "$MOONBOOK_FULL_COPY_PROJECT" ]] || full_copy_die "cleanup confirmation must exactly equal the project name"
  "${compose_base[@]}" down --volumes --remove-orphans
  printf 'removed isolated project %s; container and volume data are not recoverable\n' "$MOONBOOK_FULL_COPY_PROJECT"
  exit 0
fi

existing_containers="$(docker ps -aq --filter "label=com.docker.compose.project=$MOONBOOK_FULL_COPY_PROJECT")"
[[ -z "$existing_containers" ]] || full_copy_die "isolated project already has containers; inspect or use explicit --cleanup"

printf 'backup=%s\nsha256=%s\nproject=%s\nwork_dir=%s\nfree_bytes=%s\n' \
  "$backup_path" "$actual_sha" "$MOONBOOK_FULL_COPY_PROJECT" "$work_dir" "$(full_copy_disk_bytes "$(dirname "$work_dir")")"

if [[ "$action" == "--preflight" ]]; then
  printf 'preflight=passed\n'
  exit 0
fi

# The rehearsal env file is sourced only after path and project validation.
set -a
# shellcheck disable=SC1090
source "$MOONBOOK_FULL_COPY_ENV_FILE"
set +a
for name in POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD REDIS_PASSWORD MINIO_ROOT_USER MINIO_ROOT_PASSWORD MINIO_BUCKET; do
  [[ -n "${!name:-}" ]] || full_copy_die "environment file is missing: $name"
done
legacy_database="${MOONBOOK_FULL_COPY_MYSQL_DATABASE:-moonbook_admin}"
legacy_root_password="${MOONBOOK_FULL_COPY_MYSQL_ROOT_PASSWORD:-}"
legacy_read_password="${MOONBOOK_FULL_COPY_MYSQL_READ_PASSWORD:-}"
full_copy_validate_safe_value POSTGRES_DB "$POSTGRES_DB"
full_copy_validate_safe_value POSTGRES_USER "$POSTGRES_USER"
full_copy_validate_safe_value POSTGRES_PASSWORD "$POSTGRES_PASSWORD"
full_copy_validate_safe_value MOONBOOK_FULL_COPY_MYSQL_DATABASE "$legacy_database"
full_copy_validate_safe_value MOONBOOK_FULL_COPY_MYSQL_ROOT_PASSWORD "$legacy_root_password"
full_copy_validate_safe_value MOONBOOK_FULL_COPY_MYSQL_READ_PASSWORD "$legacy_read_password"

mkdir -p "$work_dir"
chmod 700 "$work_dir"
overlay="$work_dir/compose.full-copy.yaml"
cat >"$overlay" <<'YAML'
services:
  legacy-mysql:
    image: ${MYSQL_IMAGE:-mysql:8.4}
    restart: "no"
    command: ["--character-set-server=utf8mb4", "--collation-server=utf8mb4_0900_ai_ci"]
    environment:
      MYSQL_ROOT_PASSWORD: ${MOONBOOK_FULL_COPY_MYSQL_ROOT_PASSWORD:?required}
      MYSQL_DATABASE: ${MOONBOOK_FULL_COPY_MYSQL_DATABASE:-moonbook_admin}
    healthcheck:
      test: ["CMD-SHELL", "mysqladmin ping -h 127.0.0.1 -uroot -p$${MYSQL_ROOT_PASSWORD} --silent"]
      interval: 5s
      timeout: 5s
      retries: 60
      start_period: 20s
    volumes:
      - legacy_mysql_data:/var/lib/mysql
    networks:
      - backend
volumes:
  legacy_mysql_data:
YAML
chmod 600 "$overlay"
compose=("${compose_base[@]}" -f "$overlay")
export MOONBOOK_FULL_COPY_EVENT_LOG="$work_dir/events.tsv"
: >"$MOONBOOK_FULL_COPY_EVENT_LOG"
chmod 600 "$MOONBOOK_FULL_COPY_EVENT_LOG"
full_copy_record start "commit=$(git -C "$root_dir" rev-parse HEAD)"
full_copy_record disk_start "$(full_copy_disk_bytes "$work_dir")"
monitor_stop="$work_dir/.monitor-running"
monitor_output="$work_dir/resources.tsv"
: >"$monitor_stop"
: >"$monitor_output"
chmod 600 "$monitor_stop" "$monitor_output"
full_copy_monitor "$MOONBOOK_FULL_COPY_PROJECT" "$monitor_stop" "$monitor_output" &
monitor_pid=$!
stop_monitor() {
  rm -f "$monitor_stop"
  kill "$monitor_pid" 2>/dev/null || true
  wait "$monitor_pid" 2>/dev/null || true
}
trap stop_monitor EXIT

"${compose[@]}" config --quiet
"${compose[@]}" up -d --wait --wait-timeout 180 legacy-mysql postgres redis minio
"${compose[@]}" run --rm minio-init
full_copy_record infrastructure_started "$MOONBOOK_FULL_COPY_PROJECT"

restore_started="$(full_copy_epoch)"
restore_bytes_file="$work_dir/restored-bytes.txt"
gzip -cd "$backup_path" | tee >(wc -c >"$restore_bytes_file") | "${compose[@]}" exec -T -e MYSQL_PWD="$legacy_root_password" legacy-mysql mysql --user=root --default-character-set=utf8mb4 "$legacy_database"
full_copy_record mysql_restore_seconds "$(( $(full_copy_epoch) - restore_started ))"
for _ in {1..50}; do
  [[ -s "$restore_bytes_file" ]] && break
  sleep 0.1
done
restored_bytes="$(awk '{$1=$1; print}' "$restore_bytes_file")"
[[ "$restored_bytes" =~ ^[0-9]+$ ]] || full_copy_die "restored byte count is not numeric"
full_copy_record mysql_restore_bytes "$restored_bytes"

"${compose[@]}" exec -T -e MYSQL_PWD="$legacy_root_password" legacy-mysql mysql --user=root --database="$legacy_database" --execute="CREATE USER IF NOT EXISTS 'moonbook_migration'@'%' IDENTIFIED BY '$legacy_read_password'; GRANT SELECT, SHOW VIEW ON \`$legacy_database\`.* TO 'moonbook_migration'@'%'; SET GLOBAL read_only=ON; SET GLOBAL super_read_only=ON;"
source_before="$work_dir/source-before.tsv"
MYSQL_DOCKER_CONTAINER="${MOONBOOK_FULL_COPY_PROJECT}-legacy-mysql-1" MYSQL_USER=moonbook_migration MYSQL_PASSWORD="$legacy_read_password" MYSQL_DATABASE="$legacy_database" "$root_dir/scripts/inventory-legacy-mysql.sh" "$source_before"
full_copy_record source_inventory_before "$(full_copy_sha256 "$source_before")"

txt_table_exists="$("${compose[@]}" exec -T -e MYSQL_PWD="$legacy_read_password" legacy-mysql mysql --batch --raw --skip-column-names --user=moonbook_migration --database="$legacy_database" --execute="SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='novel_txt_import_task'")"
txt_task_count=0
if [[ "$txt_table_exists" == "1" ]]; then
  txt_task_count="$("${compose[@]}" exec -T -e MYSQL_PWD="$legacy_read_password" legacy-mysql mysql --batch --raw --skip-column-names --user=moonbook_migration --database="$legacy_database" --execute="SELECT COUNT(*) FROM novel_txt_import_task")"
fi
[[ "$txt_task_count" =~ ^[0-9]+$ ]] || full_copy_die "legacy TXT task count is not numeric"
full_copy_record legacy_txt_task_count "$txt_task_count"

if (( txt_task_count > 0 )); then
  for name in MOONBOOK_FULL_COPY_TXT_MANIFEST MOONBOOK_FULL_COPY_TXT_ROOT; do
    [[ -n "${!name:-}" ]] || full_copy_die "source has $txt_task_count TXT tasks; missing required run variable: $name"
  done
  full_copy_require_absolute_file MOONBOOK_FULL_COPY_TXT_MANIFEST "$MOONBOOK_FULL_COPY_TXT_MANIFEST"
  full_copy_require_absolute_directory MOONBOOK_FULL_COPY_TXT_ROOT "$MOONBOOK_FULL_COPY_TXT_ROOT"
  txt_manifest="$MOONBOOK_FULL_COPY_TXT_MANIFEST"
  txt_root="$MOONBOOK_FULL_COPY_TXT_ROOT"
  full_copy_record legacy_txt_manifest external
else
  txt_root="$work_dir/empty-txt-root"
  txt_manifest="$work_dir/empty-legacy-txt-manifest.json"
  mkdir -p "$txt_root"
  printf '{"root":"empty-txt-root","files":{}}\n' >"$txt_manifest"
  chmod 600 "$txt_manifest"
  full_copy_record legacy_txt_manifest generated_empty
fi

"${compose[@]}" run --rm --build migrate
target_dsn="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@postgres:5432/$POSTGRES_DB?sslmode=disable"
legacy_dsn="moonbook_migration:$legacy_read_password@tcp(legacy-mysql:3306)/$legacy_database?charset=utf8mb4&parseTime=true&loc=UTC"
migration_started="$(full_copy_epoch)"
"${compose[@]}" run --rm --no-deps \
  -v "$txt_manifest:$txt_manifest:ro" \
  -v "$txt_root:$txt_root:ro" \
  -e MOONBOOK_LEGACY_MYSQL_DSN="$legacy_dsn" \
  -e MOONBOOK_DATABASE_DSN="$target_dsn" \
  -e MOONBOOK_LEGACY_TXT_MANIFEST="$txt_manifest" \
  -e MOONBOOK_MIGRATION_TIMEOUT=12h \
  server moonbook-legacy-migrate all
full_copy_record migration_seconds "$(( $(full_copy_epoch) - migration_started ))"

audit_report="$work_dir/migration-audit.json"
audit_started="$(full_copy_epoch)"
"${compose[@]}" run --rm --no-deps \
  -e MOONBOOK_LEGACY_MYSQL_DSN="$legacy_dsn" \
  -e MOONBOOK_DATABASE_DSN="$target_dsn" \
  -e MOONBOOK_AUDIT_TIMEOUT=12h \
  server moonbook-migration-audit moonbook-v1 >"$audit_report"
chmod 600 "$audit_report"
full_copy_record audit_seconds "$(( $(full_copy_epoch) - audit_started ))"
full_copy_record audit_sha256 "$(full_copy_sha256 "$audit_report")"

rerun_started="$(full_copy_epoch)"
structure_rerun="$work_dir/structure-rerun.txt"
"${compose[@]}" run --rm migrate >"$structure_rerun"
grep -qx 'applied=0' "$structure_rerun" || full_copy_die "structure rerun applied unexpected migrations"
full_copy_record structure_rerun applied_0
"${compose[@]}" run --rm --no-deps \
  -v "$txt_manifest:$txt_manifest:ro" \
  -v "$txt_root:$txt_root:ro" \
  -e MOONBOOK_LEGACY_MYSQL_DSN="$legacy_dsn" \
  -e MOONBOOK_DATABASE_DSN="$target_dsn" \
  -e MOONBOOK_LEGACY_TXT_MANIFEST="$txt_manifest" \
  -e MOONBOOK_MIGRATION_TIMEOUT=12h \
  server moonbook-legacy-migrate all
full_copy_record rerun_seconds "$(( $(full_copy_epoch) - rerun_started ))"

audit_rerun_report="$work_dir/migration-audit-rerun.json"
"${compose[@]}" run --rm --no-deps \
  -e MOONBOOK_LEGACY_MYSQL_DSN="$legacy_dsn" \
  -e MOONBOOK_DATABASE_DSN="$target_dsn" \
  -e MOONBOOK_AUDIT_TIMEOUT=12h \
  server moonbook-migration-audit moonbook-v1 >"$audit_rerun_report"
chmod 600 "$audit_rerun_report"
sed '/"generatedAt":/d' "$audit_report" >"$work_dir/migration-audit.stable.json"
sed '/"generatedAt":/d' "$audit_rerun_report" >"$work_dir/migration-audit-rerun.stable.json"
cmp -s "$work_dir/migration-audit.stable.json" "$work_dir/migration-audit-rerun.stable.json" || full_copy_die "business or audit results changed after idempotent rerun"
full_copy_record audit_rerun_sha256 "$(full_copy_sha256 "$work_dir/migration-audit-rerun.stable.json")"

source_after="$work_dir/source-after.tsv"
MYSQL_DOCKER_CONTAINER="${MOONBOOK_FULL_COPY_PROJECT}-legacy-mysql-1" MYSQL_USER=moonbook_migration MYSQL_PASSWORD="$legacy_read_password" MYSQL_DATABASE="$legacy_database" "$root_dir/scripts/inventory-legacy-mysql.sh" "$source_after"
cmp -s "$source_before" "$source_after" || full_copy_die "source inventory changed during rehearsal"
full_copy_record source_inventory_after "$(full_copy_sha256 "$source_after")"
full_copy_record disk_end "$(full_copy_disk_bytes "$work_dir")"
stop_monitor
trap - EXIT
full_copy_record resources_sha256 "$(full_copy_sha256 "$monitor_output")"
full_copy_record complete "report=$audit_report"
printf 'full-copy rehearsal completed; evidence is in %s\n' "$work_dir"
printf 'resources remain stopped/running for inspection; use explicit --cleanup after review\n'
