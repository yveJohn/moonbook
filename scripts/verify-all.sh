#!/usr/bin/env bash
set -uo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
all_stages="quality management reader integration migration e2e compose monitoring security"
selected_stages="${MOONBOOK_VERIFY_STAGES:-}"
[[ -n "$selected_stages" ]] || selected_stages="$all_stages"
export GOCACHE="${GOCACHE:-/tmp/moonbook-go-cache}"

for command in git jq; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 2; }
done

is_known_stage() {
  case " $all_stages " in
    *" $1 "*) return 0 ;;
    *) return 1 ;;
  esac
}

for stage in $selected_stages; do
  is_known_stage "$stage" || { echo "unknown verification stage: $stage" >&2; exit 2; }
done

output_base="${MOONBOOK_VERIFY_OUTPUT_DIR:-}"
if [[ -z "$output_base" ]]; then
  output_dir="$(mktemp -d "${TMPDIR:-/tmp}/moonbook-verify.XXXXXX")"
else
  [[ "$output_base" == /* ]] || { echo "MOONBOOK_VERIFY_OUTPUT_DIR must be absolute" >&2; exit 2; }
  mkdir -p "$output_base"
  output_base="$(cd "$output_base" && pwd -P)"
  output_dir="$(mktemp -d "$output_base/moonbook-verify.XXXXXX")"
fi
output_dir="$(cd "$output_dir" && pwd -P)"
summary_dir="$output_dir/summaries"
mkdir -p "$summary_dir"

commit="$(git -C "$root_dir" rev-parse HEAD)"
run_started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
overall_status=0

jq -n \
  --arg commit "$commit" \
  --arg go "$(go version 2>/dev/null || true)" \
  --arg node "$(node --version 2>/dev/null || true)" \
  --arg npm "$(npm --version 2>/dev/null || true)" \
  --arg pnpm "$(pnpm --version 2>/dev/null || true)" \
  --arg docker "$(docker version --format '{{.Client.Version}}' 2>/dev/null || true)" \
  '{commit:$commit,go:$go,node:$node,npm:$npm,pnpm:$pnpm,dockerClient:$docker}' \
  >"$output_dir/tools.json"

write_stage_summary() {
  local stage="$1" started="$2" finished="$3" duration="$4" status="$5"
  jq -n \
    --arg stage "$stage" \
    --arg commit "$commit" \
    --arg startedAt "$started" \
    --arg finishedAt "$finished" \
    --argjson durationSeconds "$duration" \
    --argjson exitStatus "$status" \
    '{stage:$stage,commit:$commit,startedAt:$startedAt,finishedAt:$finishedAt,durationSeconds:$durationSeconds,exitStatus:$exitStatus,result:(if $exitStatus == 0 then "passed" else "failed" end)}' \
    >"$summary_dir/$stage.json"
}

run_stage_command() {
  local stage="$1"
  case "$stage" in
    quality)
      (
        cd "$root_dir/server"
        test -z "$(gofmt -l internal/modules internal/platform cmd/moonbook-admin cmd/moonbook-browser-fixture cmd/moonbook-config cmd/moonbook-finance-reconcile cmd/moonbook-legacy-migrate cmd/moonbook-migrate cmd/moonbook-migration-audit)" &&
        go mod verify &&
        CGO_ENABLED=0 go vet ./internal/... ./config ./core/... ./initialize/... ./middleware ./cmd/moonbook-admin ./cmd/moonbook-browser-fixture ./cmd/moonbook-config ./cmd/moonbook-finance-reconcile ./cmd/moonbook-legacy-migrate ./cmd/moonbook-migrate ./cmd/moonbook-migration-audit &&
        if [[ "$(go env GOOS)" == "linux" ]]; then
          go test -race ./internal/... ./config ./core/... ./initialize/... ./middleware ./cmd/moonbook-admin ./cmd/moonbook-browser-fixture ./cmd/moonbook-config ./cmd/moonbook-finance-reconcile ./cmd/moonbook-legacy-migrate ./cmd/moonbook-migrate ./cmd/moonbook-migration-audit
        else
          echo "race detector runs in Linux CI; running CGO-disabled tests on $(go env GOOS)"
          CGO_ENABLED=0 go test ./internal/... ./config ./core/... ./initialize/... ./middleware ./cmd/moonbook-admin ./cmd/moonbook-browser-fixture ./cmd/moonbook-config ./cmd/moonbook-finance-reconcile ./cmd/moonbook-legacy-migrate ./cmd/moonbook-migrate ./cmd/moonbook-migration-audit
        fi
      )
      ;;
    management)
      (cd "$root_dir/web" && pnpm install --frozen-lockfile && pnpm run verify:management)
      ;;
    reader)
      (cd "$root_dir/reader-ui" && npm ci) &&
        "$root_dir/scripts/verify-m3.sh" --reader-only
      ;;
    integration)
      "$root_dir/scripts/verify-m3.sh" --integration-only
      ;;
    migration)
      "$root_dir/scripts/verify-migrations.sh"
      ;;
    e2e)
      "$root_dir/scripts/verify-management-e2e.sh"
      ;;
    compose)
      "$root_dir/scripts/verify-m1.sh"
      ;;
    monitoring)
      "$root_dir/scripts/verify-monitoring.sh" --config "${MOONBOOK_ENV_FILE:-$root_dir/.env}"
      ;;
    security)
      MOONBOOK_SECURITY_OUTPUT_DIR="$output_dir/security" "$root_dir/scripts/verify-security.sh"
      ;;
  esac
}

for stage in $selected_stages; do
  echo "==> verification stage: $stage"
  stage_started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  start_seconds="$(date +%s)"
  set +e
  run_stage_command "$stage"
  status=$?
  set -e
  finish_seconds="$(date +%s)"
  stage_finished="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  write_stage_summary "$stage" "$stage_started" "$stage_finished" "$((finish_seconds - start_seconds))" "$status"
  if (( status != 0 )); then
    overall_status=1
    echo "verification stage failed: $stage (exit=$status)" >&2
  fi
done

jq -s \
  --arg commit "$commit" \
  --arg startedAt "$run_started" \
  --arg finishedAt "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  '{commit:$commit,startedAt:$startedAt,finishedAt:$finishedAt,result:(if all(.[]; .exitStatus == 0) then "passed" else "failed" end),stages:.}' \
  "$summary_dir"/*.json >"$output_dir/summary.json"

printf 'verification summary: %s\n' "$output_dir/summary.json"
exit "$overall_status"
