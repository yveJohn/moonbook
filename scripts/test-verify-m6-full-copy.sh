#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fixture_dir="$(mktemp -d)"
trap 'rm -rf "$fixture_dir"' EXIT

mkdir -p "$fixture_dir/bin" "$fixture_dir/work"
printf 'SELECT 1;\n' | gzip >"$fixture_dir/backup.sql.gz"
printf 'POSTGRES_DB=test\n' >"$fixture_dir/rehearsal.env"
expected_sha="$(shasum -a 256 "$fixture_dir/backup.sql.gz" | awk '{print $1}')"

cat >"$fixture_dir/bin/docker" <<'SH'
#!/usr/bin/env bash
if [[ "${1:-}" == "compose" && "${2:-}" == "version" ]]; then
  exit 0
fi
if [[ "${1:-}" == "ps" ]]; then
  exit 0
fi
exit 99
SH
chmod +x "$fixture_dir/bin/docker"

base_env=(
  PATH="$fixture_dir/bin:$PATH"
  MOONBOOK_FULL_COPY_BACKUP="$fixture_dir/backup.sql.gz"
  MOONBOOK_FULL_COPY_SHA256="$expected_sha"
  MOONBOOK_FULL_COPY_PROJECT=moonbook_verify_m6_contract
  MOONBOOK_FULL_COPY_ENV_FILE="$fixture_dir/rehearsal.env"
  MOONBOOK_FULL_COPY_WORK_DIR="$fixture_dir/work"
  MOONBOOK_FULL_COPY_CONFIRM=M6_FULL_COPY_LOCAL_ISOLATED
)

assert_rejected() {
  local expected="$1"
  shift
  local output
  if output="$(env "${base_env[@]}" "$@" 2>&1)"; then
    printf 'expected rejection containing %q\n' "$expected" >&2
    exit 1
  fi
  [[ "$output" == *"$expected"* ]] || {
    printf 'missing rejection %q in: %s\n' "$expected" "$output" >&2
    exit 1
  }
}

assert_rejected "must be an absolute path" env MOONBOOK_FULL_COPY_BACKUP=relative.sql.gz "$root_dir/scripts/verify-m6-full-copy.sh" --preflight
assert_rejected "backup SHA-256 does not match" env MOONBOOK_FULL_COPY_SHA256="$(printf '0%.0s' {1..64})" "$root_dir/scripts/verify-m6-full-copy.sh" --preflight
assert_rejected "project must match" env MOONBOOK_FULL_COPY_PROJECT=moonbook "$root_dir/scripts/verify-m6-full-copy.sh" --preflight
mkdir -p "$root_dir/.m6-contract-work"
assert_rejected "outside the active and frozen repositories" env MOONBOOK_FULL_COPY_WORK_DIR="$root_dir/.m6-contract-work" "$root_dir/scripts/verify-m6-full-copy.sh" --preflight
rmdir "$root_dir/.m6-contract-work"
assert_rejected "cleanup confirmation must exactly equal" env MOONBOOK_FULL_COPY_CLEANUP_CONFIRM=wrong "$root_dir/scripts/verify-m6-full-copy.sh" --cleanup

env "${base_env[@]}" "$root_dir/scripts/verify-m6-full-copy.sh" --preflight >/dev/null
printf 'M6 full-copy script contract tests passed.\n'
