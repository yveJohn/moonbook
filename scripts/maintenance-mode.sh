#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
action="${1:-}"
shift || true
env_file="${MOONBOOK_ENV_FILE:-$root_dir/.env}"
confirmed_project=""

while (($#)); do
  case "$1" in
    --env-file) env_file="${2:-}"; shift 2 ;;
    --confirm-project) confirmed_project="${2:-}"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

case "$action" in on|off|status) ;; *)
  echo "usage: scripts/maintenance-mode.sh on|off|status [--env-file PATH] [--confirm-project NAME]" >&2
  exit 2
esac
[[ "$env_file" == /* ]] || env_file="$root_dir/$env_file"
[[ -f "$env_file" ]] || { echo "environment file not found: $env_file" >&2; exit 2; }
for command in docker node curl; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 2; }
done

compose=(docker compose --project-directory "$root_dir" --env-file "$env_file" -f "$root_dir/compose.yaml")
maintenance_compose=("${compose[@]}" -f "$root_dir/deploy/compose/compose.maintenance.yaml")
project="$("${compose[@]}" config --format json | node -e 'let s="";process.stdin.on("data",d=>s+=d);process.stdin.on("end",()=>process.stdout.write(JSON.parse(s).name))')"

if [[ "$action" != status && "$confirmed_project" != "$project" ]]; then
  echo "refusing to change project '$project'; pass --confirm-project '$project'" >&2
  exit 2
fi

gateway_url() {
  local target="$1" mapping
  mapping="$("${compose[@]}" port gateway "$target")"
  printf 'http://127.0.0.1:%s' "${mapping##*:}"
}

wait_for_status() {
  local url="$1" expected="$2" code=""
  for _ in {1..30}; do
    code="$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
    [[ "$code" == "$expected" ]] && return 0
    sleep 1
  done
  echo "timed out waiting for HTTP $expected from $url (last status: ${code:-none})" >&2
  return 1
}

admin_url="$(gateway_url 8080)"
reader_url="$(gateway_url 8081)"

case "$action" in
  on)
    "${maintenance_compose[@]}" up -d --no-deps --force-recreate gateway
    wait_for_status "$admin_url/gateway-health" 200
    for url in "$admin_url/" "$reader_url/" "$admin_url/api/health/ready"; do
      headers="$(curl -sS -D - -o /dev/null "$url")"
      grep -qE '^HTTP/[^ ]+ 503' <<<"$headers"
      grep -qiE '^Retry-After: 300\r?$' <<<"$headers"
    done
    "${compose[@]}" stop -t 30 server reader-ui web
    running="$("${compose[@]}" ps --status running --services)"
    for service in server reader-ui web; do
      if grep -qx "$service" <<<"$running"; then
        echo "$service is still running after maintenance entry" >&2
        exit 1
      fi
    done
    echo "maintenance mode enabled for project $project; gateway returns 503 and data services remain running"
    ;;
  off)
    "${compose[@]}" up -d --wait --wait-timeout 180 server web reader-ui
    if ! "${compose[@]}" up -d --no-deps --force-recreate gateway \
      || ! wait_for_status "$admin_url/gateway-health" 200 \
      || ! wait_for_status "$admin_url/" 200 \
      || ! wait_for_status "$reader_url/health" 200; then
      echo "normal gateway verification failed; restoring maintenance gateway" >&2
      "${maintenance_compose[@]}" up -d --no-deps --force-recreate gateway >/dev/null 2>&1 || true
      exit 1
    fi
    echo "maintenance mode disabled for project $project; application endpoints are healthy"
    ;;
  status)
    code="$(curl -sS -o /dev/null -w '%{http_code}' "$admin_url/" 2>/dev/null || true)"
    case "$code" in
      503) echo "maintenance" ;;
      200) echo "active" ;;
      *) echo "unknown (HTTP ${code:-unreachable})"; exit 1 ;;
    esac
    ;;
esac
