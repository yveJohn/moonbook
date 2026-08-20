#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
action="${1:---config}"
env_file="${2:-$root_dir/.env}"
case "$action" in
  --config|--inject) ;;
  *) echo "usage: scripts/verify-monitoring.sh [--config|--inject] [env-file]" >&2; exit 2 ;;
esac
[[ "$env_file" == /* ]] || env_file="$root_dir/$env_file"
[[ -f "$env_file" ]] || { echo "monitoring env file not found: $env_file" >&2; exit 2; }

for command in docker curl awk sed; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 2; }
done

set -a
# shellcheck disable=SC1090
source "$env_file"
set +a
[[ -n "${POSTGRES_EXPORTER_PASSWORD:-}" && "$POSTGRES_EXPORTER_PASSWORD" != "monitoring_not_configured" ]] || {
  echo "POSTGRES_EXPORTER_PASSWORD must be configured" >&2
  exit 2
}
token_file="${MOONBOOK_METRICS_TOKEN_FILE:-$root_dir/.local/moonbook-metrics-token}"
[[ "$token_file" == /* ]] || token_file="$root_dir/$token_file"
[[ -f "$token_file" && ! -L "$token_file" ]] || { echo "metrics token file must be a regular non-symlink file" >&2; exit 2; }
token_file_value="$(tr -d '\r\n' <"$token_file")"
[[ -n "$token_file_value" && "$token_file_value" == "${MOONBOOK_METRICS_TOKEN:-}" ]] || {
  echo "metrics token file must exactly match MOONBOOK_METRICS_TOKEN" >&2
  exit 2
}

compose=(docker compose --project-directory "$root_dir" --env-file "$env_file" --profile monitoring)
"${compose[@]}" config --quiet
docker run --rm --entrypoint /bin/promtool \
  -v "$root_dir/deploy/monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro" \
  -v "$root_dir/deploy/monitoring/alerts:/etc/prometheus/alerts:ro" \
  -v "$token_file:/run/secrets/moonbook_metrics_token:ro" \
  "${PROMETHEUS_IMAGE:-prom/prometheus:v3.5.0@sha256:63805ebb8d2b3920190daf1cb14a60871b16fd38bed42b857a3182bc621f4996}" check config /etc/prometheus/prometheus.yml
docker run --rm --entrypoint /bin/promtool \
  -v "$root_dir/deploy/monitoring/alerts:/etc/prometheus/alerts:ro" \
  "${PROMETHEUS_IMAGE:-prom/prometheus:v3.5.0@sha256:63805ebb8d2b3920190daf1cb14a60871b16fd38bed42b857a3182bc621f4996}" check rules /etc/prometheus/alerts/moonbook.yml
docker run --rm --entrypoint /bin/amtool \
  -v "$root_dir/deploy/monitoring/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro" \
  "${ALERTMANAGER_IMAGE:-prom/alertmanager:v0.28.1@sha256:27c475db5fb156cab31d5c18a4251ac7ed567746a2483ff264516437a39b15ba}" check-config /etc/alertmanager/alertmanager.yml

if [[ "$action" == "--config" ]]; then
  echo "monitoring configuration verification passed"
  exit 0
fi

prometheus_port="${PROMETHEUS_HOST_PORT:-19090}"
webhook_port="${MONITORING_WEBHOOK_HOST_PORT:-19094}"
"${compose[@]}" up -d --build

wait_http() {
  local url="$1"
  for _ in {1..90}; do
    curl -fsS "$url" >/dev/null 2>&1 && return 0
    sleep 2
  done
  echo "timed out waiting for $url" >&2
  return 1
}
wait_http "http://127.0.0.1:$prometheus_port/-/ready"
wait_http "http://127.0.0.1:$webhook_port/health"

for _ in {1..45}; do
  up_response="$(curl -fsS --get --data-urlencode 'query=up' "http://127.0.0.1:$prometheus_port/api/v1/query")"
  ready=true
  for job in blackbox moonbook-server postgres redis minio cadvisor; do
    [[ "$up_response" == *"\"job\":\"$job\""* ]] || ready=false
  done
  [[ "$up_response" != *'\"0\"'* ]] || ready=false
  [[ "$ready" == true ]] && break
  sleep 2
done
[[ "$ready" == true ]] || { echo "not all required monitoring targets are up" >&2; exit 1; }

query_alert() {
  curl -fsS --get --data-urlencode 'query=ALERTS{alertname="MoonbookEndpointDown",alertstate="firing"}' \
    "http://127.0.0.1:$prometheus_port/api/v1/query"
}
event_count() {
  local field="$1"
  curl -fsS "http://127.0.0.1:$webhook_port/events" | sed -n "s/.*\"$field\":\([0-9][0-9]*\).*/\1/p"
}

initial_firing="$(event_count firing)"
initial_resolved="$(event_count resolved)"
initial_firing="${initial_firing:-0}"
initial_resolved="${initial_resolved:-0}"
restored=false
restore_reader() {
  if [[ "$restored" != true ]]; then
    "${compose[@]}" up -d reader-ui >/dev/null 2>&1 || true
  fi
}
trap restore_reader EXIT

"${compose[@]}" stop reader-ui
for _ in {1..30}; do
  if query_alert | grep -q 'MoonbookEndpointDown'; then
    break
  fi
  sleep 5
done
query_alert | grep -q 'MoonbookEndpointDown' || { echo "endpoint-down alert did not fire" >&2; exit 1; }
for _ in {1..30}; do
  current="$(event_count firing)"
  if (( ${current:-0} > initial_firing )); then
    break
  fi
  sleep 2
done
current="$(event_count firing)"
(( ${current:-0} > initial_firing )) || { echo "firing notification did not reach webhook" >&2; exit 1; }

"${compose[@]}" up -d reader-ui
restored=true
for _ in {1..45}; do
  current="$(event_count resolved)"
  if (( ${current:-0} > initial_resolved )); then
    break
  fi
  sleep 2
done
current="$(event_count resolved)"
(( ${current:-0} > initial_resolved )) || { echo "resolved notification did not reach webhook" >&2; exit 1; }
trap - EXIT
echo "monitoring fault injection passed: firing and resolved notifications reached the local webhook"
