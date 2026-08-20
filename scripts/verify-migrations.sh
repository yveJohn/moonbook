#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
admin_dsn="${MOONBOOK_MIGRATION_TEST_ADMIN_DSN:-}"

for command in go node; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 2; }
done
[[ -n "$admin_dsn" ]] || { echo "MOONBOOK_MIGRATION_TEST_ADMIN_DSN is required" >&2; exit 2; }
node -e '
  const value = process.argv[1]
  const parsed = new URL(value)
  if (!["postgres:", "postgresql:"].includes(parsed.protocol)) throw new Error("migration DSN must use PostgreSQL")
  if (!["127.0.0.1", "::1", "localhost"].includes(parsed.hostname)) throw new Error("migration DSN must use a loopback host")
' "$admin_dsn"

(
  cd "$root_dir/server"
  GOCACHE="${GOCACHE:-/tmp/moonbook-go-cache}" CGO_ENABLED=0 go test \
    ./internal/platform/migrate \
    ./internal/platform/legacymigrate \
    ./internal/platform/legacyaudit
  GOCACHE="${GOCACHE:-/tmp/moonbook-go-cache}" CGO_ENABLED=0 go test \
    -tags=integration -count=1 -p=1 \
    ./internal/platform/migrate
)

"$root_dir/scripts/test-verify-m6-full-copy.sh"
echo "migration verification passed: unit, PostgreSQL integration, replay, and full-copy contracts"
