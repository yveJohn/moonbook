#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fixture_dir="$(mktemp -d "${TMPDIR:-/tmp}/moonbook-deploy-test.XXXXXX")"
fixture_dir="$(cd "$fixture_dir" && pwd -P)"
trap 'rm -rf "$fixture_dir"' EXIT

fake_bin="$fixture_dir/bin"
log_file="$fixture_dir/commands.log"
mkdir -p "$fake_bin"
: >"$log_file"

cat >"$fake_bin/git" <<'SH'
#!/usr/bin/env bash
case " $* " in
  *" rev-parse --show-toplevel "*) printf '%s\n' "$MOONBOOK_TEST_ROOT" ;;
  *" rev-parse --short=12 HEAD "*) printf 'abcdef123456\n' ;;
  *" rev-parse HEAD "*) printf 'abcdef1234567890abcdef1234567890abcdef12\n' ;;
  *" status --porcelain "*) [[ "${MOONBOOK_TEST_GIT_DIRTY:-0}" == 1 ]] && printf ' M dirty-file\n' ;;
  *) printf 'unexpected git command: %s\n' "$*" >&2; exit 90 ;;
esac
SH

cat >"$fake_bin/docker" <<'SH'
#!/usr/bin/env bash
printf 'docker %s\n' "$*" >>"$MOONBOOK_TEST_LOG"
case "${1:-} ${2:-}" in
  "buildx version"|"buildx build") exit 0 ;;
  "image inspect") printf 'sha256:test-image-id\n'; exit 0 ;;
esac
if [[ "${1:-}" == save ]]; then
  printf 'test-image-archive'
  exit 0
fi
printf 'unexpected docker command: %s\n' "$*" >&2
exit 91
SH

cat >"$fake_bin/ssh" <<'SH'
#!/usr/bin/env bash
printf 'ssh %s\n' "$*" >>"$MOONBOOK_TEST_LOG"
stage=""
previous=""
for argument in "$@"; do
  if [[ "$previous" == -- ]]; then
    stage="$argument"
    break
  fi
  previous="$argument"
done
cat >/dev/null
printf 'remote-stage %s\n' "$stage" >>"$MOONBOOK_TEST_LOG"
if [[ "${MOONBOOK_TEST_FAIL_STAGE:-}" == "$stage" ]]; then
  printf 'forced remote failure: %s\n' "$stage" >&2
  exit 92
fi
if [[ "$stage" == prepare ]]; then
  printf 'previous_server_image=moonbook/server:old\n'
  printf 'previous_web_image=moonbook/web:old\n'
  printf 'previous_reader_image=moonbook/reader-ui:old\n'
fi
SH

cat >"$fake_bin/scp" <<'SH'
#!/usr/bin/env bash
printf 'scp %s\n' "$*" >>"$MOONBOOK_TEST_LOG"
SH

chmod 0755 "$fake_bin/git" "$fake_bin/docker" "$fake_bin/ssh" "$fake_bin/scp"

base_env=(
  PATH="$fake_bin:$PATH"
  MOONBOOK_TEST_ROOT="$root_dir"
  MOONBOOK_TEST_LOG="$log_file"
  MOONBOOK_APP_MASTER_KEY="unit-test-super-secret"
)
script="$root_dir/scripts/deploy-production.sh"
target="root@101.32.210.229:/opt/moonbook-app"

fail() {
  printf 'deploy script test failed: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local haystack="$1" needle="$2"
  [[ "$haystack" == *"$needle"* ]] || fail "missing '$needle' in: $haystack"
}

assert_not_contains() {
  local haystack="$1" needle="$2"
  [[ "$haystack" != *"$needle"* ]] || fail "unexpected '$needle' in: $haystack"
}

assert_rejected() {
  local expected="$1"
  shift
  local output
  if output="$(env "${base_env[@]}" "$@" 2>&1)"; then
    fail "command unexpectedly succeeded: $*"
  fi
  assert_contains "$output" "$expected"
}

help_output="$($script --help)"
assert_contains "$help_output" "--build-only"
assert_contains "$help_output" "--confirm-target"
assert_rejected "unknown argument" "$script" --unknown
assert_rejected "--host requires a value" "$script" --host
assert_rejected "cannot be combined" "$script" --dry-run --build-only
assert_rejected "remote directory must be an absolute path" "$script" --dry-run --remote-dir relative
assert_rejected "output directory must be outside the repository" "$script" --build-only --output-dir "$root_dir/artifacts"
assert_rejected "refusing production deployment" "$script"

: >"$log_file"
dry_output="$(env "${base_env[@]}" "$script" --dry-run)"
assert_contains "$dry_output" "mode=dry-run"
assert_contains "$dry_output" "commit=abcdef1234567890abcdef1234567890abcdef12"
assert_contains "$dry_output" "target=$target"
assert_contains "$dry_output" "Docker, SSH and SCP were not invoked"
[[ ! -s "$log_file" ]] || fail "dry-run invoked an external deployment command"

assert_rejected "Git worktree is dirty" env MOONBOOK_TEST_GIT_DIRTY=1 "$script" --dry-run

: >"$log_file"
build_dir="$fixture_dir/build-only"
build_output="$(env "${base_env[@]}" "$script" --build-only --output-dir "$build_dir")"
assert_contains "$build_output" "build-only complete: $build_dir"
[[ -s "$build_dir/moonbook-images-abcdef123456.tar.gz" ]] || fail "build archive was not created"
[[ -s "$build_dir/moonbook-images-abcdef123456.tar.gz.sha256" ]] || fail "checksum was not created"
[[ -s "$build_dir/release-abcdef123456.txt" ]] || fail "release manifest was not created"
build_log="$(<"$log_file")"
assert_contains "$build_log" "moonbook/server:abcdef123456"
assert_contains "$build_log" "moonbook/web:abcdef123456"
assert_contains "$build_log" "moonbook/reader-ui:abcdef123456"
assert_not_contains "$build_log" "ssh "
assert_not_contains "$build_log" "scp "

: >"$log_file"
publish_dir="$fixture_dir/publish"
publish_output="$(env "${base_env[@]}" "$script" --output-dir "$publish_dir" --confirm-target "$target")"
assert_contains "$publish_output" "deployment complete"
assert_contains "$publish_output" "production health checks passed"
assert_contains "$publish_output" "compose.release.previous.yml"
assert_not_contains "$publish_output" "unit-test-super-secret"
publish_log="$(<"$log_file")"
assert_contains "$publish_log" "remote-stage preflight"
assert_contains "$publish_log" "scp "
assert_contains "$publish_log" "remote-stage import"
assert_contains "$publish_log" "remote-stage prepare"
assert_contains "$publish_log" "remote-stage migrate"
assert_contains "$publish_log" "remote-stage recreate"
assert_contains "$publish_log" "remote-stage health"
assert_contains "$publish_log" "remote-stage cleanup"
preflight_line="$(grep -n 'remote-stage preflight' "$log_file" | head -n1 | cut -d: -f1)"
scp_line="$(grep -n '^scp ' "$log_file" | head -n1 | cut -d: -f1)"
import_line="$(grep -n 'remote-stage import' "$log_file" | head -n1 | cut -d: -f1)"
prepare_line="$(grep -n 'remote-stage prepare' "$log_file" | head -n1 | cut -d: -f1)"
migrate_line="$(grep -n 'remote-stage migrate' "$log_file" | head -n1 | cut -d: -f1)"
recreate_line="$(grep -n 'remote-stage recreate' "$log_file" | head -n1 | cut -d: -f1)"
health_line="$(grep -n 'remote-stage health' "$log_file" | head -n1 | cut -d: -f1)"
((preflight_line < scp_line && scp_line < import_line && import_line < prepare_line && prepare_line < migrate_line && migrate_line < recreate_line && recreate_line < health_line)) || fail "remote stages ran out of order"

: >"$log_file"
failure_dir="$fixture_dir/import-failure"
assert_rejected "forced remote failure: import" env MOONBOOK_TEST_FAIL_STAGE=import "$script" \
  --output-dir "$failure_dir" --confirm-target "$target"
failure_log="$(<"$log_file")"
assert_contains "$failure_log" "remote-stage import"
assert_contains "$failure_log" "remote-stage cleanup"
assert_not_contains "$failure_log" "remote-stage prepare"

: >"$log_file"
migration_failure_dir="$fixture_dir/migration-failure"
assert_rejected "forced remote failure: migrate" env MOONBOOK_TEST_FAIL_STAGE=migrate "$script" \
  --output-dir "$migration_failure_dir" --confirm-target "$target"
migration_failure_log="$(<"$log_file")"
assert_contains "$migration_failure_log" "remote-stage prepare"
assert_contains "$migration_failure_log" "remote-stage migrate"
assert_contains "$migration_failure_log" "remote-stage cleanup"
assert_not_contains "$migration_failure_log" "remote-stage recreate"

: >"$log_file"
health_failure_dir="$fixture_dir/health-failure"
assert_rejected "forced remote failure: health" env MOONBOOK_TEST_FAIL_STAGE=health "$script" \
  --output-dir "$health_failure_dir" --confirm-target "$target"
health_failure_log="$(<"$log_file")"
assert_contains "$health_failure_log" "remote-stage recreate"
assert_contains "$health_failure_log" "remote-stage health"
assert_contains "$health_failure_log" "remote-stage cleanup"

bash -n "$script"
printf 'production deployment script tests passed.\n'
