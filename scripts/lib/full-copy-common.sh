#!/usr/bin/env bash

full_copy_die() {
  printf 'full-copy error: %s\n' "$*" >&2
  exit 2
}

full_copy_require_command() {
  command -v "$1" >/dev/null 2>&1 || full_copy_die "required command is missing: $1"
}

full_copy_canonical_path() {
  local input="$1"
  local directory
  directory="$(cd "$(dirname "$input")" && pwd -P)" || return 1
  printf '%s/%s\n' "$directory" "$(basename "$input")"
}

full_copy_path_within() {
  local candidate="$1"
  local parent="$2"
  [[ "$candidate" == "$parent" || "$candidate" == "$parent/"* ]]
}

full_copy_validate_project() {
  [[ "$1" =~ ^moonbook_verify_m6_[a-z0-9][a-z0-9_-]{2,40}$ ]] || \
    full_copy_die "project must match moonbook_verify_m6_[a-z0-9_-]+"
}

full_copy_validate_safe_value() {
  local name="$1"
  local value="$2"
  [[ "$value" =~ ^[A-Za-z0-9_-]+$ ]] || full_copy_die "$name must contain only letters, digits, underscore, or hyphen"
}

full_copy_require_absolute_file() {
  local name="$1"
  local value="$2"
  [[ "$value" == /* ]] || full_copy_die "$name must be an absolute path"
  [[ -f "$value" && ! -L "$value" ]] || full_copy_die "$name must be a regular non-symlink file"
}

full_copy_require_absolute_directory() {
  local name="$1"
  local value="$2"
  [[ "$value" == /* ]] || full_copy_die "$name must be an absolute path"
  [[ -d "$value" && ! -L "$value" ]] || full_copy_die "$name must be a real directory"
}

full_copy_sha256() {
  shasum -a 256 "$1" | awk '{print $1}'
}

full_copy_disk_bytes() {
  df -Pk "$1" | awk 'NR==2 {print $4 * 1024}'
}

full_copy_epoch() {
  date -u +%s
}

full_copy_record() {
  local event="$1"
  local value="$2"
  printf '%s\t%s\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$event" "$value" >>"$MOONBOOK_FULL_COPY_EVENT_LOG"
}

full_copy_monitor() {
  local project="$1"
  local stop_file="$2"
  local output="$3"
  while [[ -e "$stop_file" ]]; do
    local timestamp
    timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    docker ps -q --filter "label=com.docker.compose.project=$project" | while IFS= read -r container_id; do
      [[ -n "$container_id" ]] || continue
      docker stats --no-stream --format "$timestamp\t{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.BlockIO}}\t{{.PIDs}}" "$container_id" || true
    done
    printf '%s\thost_free_bytes\t%s\n' "$timestamp" "$(full_copy_disk_bytes "$(dirname "$output")")"
    sleep 30
  done >>"$output"
}
