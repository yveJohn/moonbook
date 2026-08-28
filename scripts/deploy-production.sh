#!/usr/bin/env bash
set -euo pipefail
umask 077

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
mode="publish"
remote_host="root@101.32.210.229"
remote_dir="/opt/moonbook-app"
platform="linux/amd64"
output_dir=""
confirmed_target=""
remote_created=0
remote_release_dir=""

usage() {
  cat <<'EOF'
Usage: scripts/deploy-production.sh [options]

Build Moonbook application images locally and deploy them to the current
production Docker Compose project.

Options:
  --dry-run                  Validate inputs and print stages without side effects
  --build-only               Build and package images without remote access
  --host USER@HOST           SSH target (default: root@101.32.210.229)
  --remote-dir ABSOLUTE_PATH Remote project directory (default: /opt/moonbook-app)
  --platform OS/ARCH         Docker build platform (default: linux/amd64)
  --output-dir ABSOLUTE_PATH Preserve build artifacts in this directory
  --confirm-target VALUE     Required for publish; must equal USER@HOST:ABSOLUTE_PATH
  -h, --help                 Show this help

Examples:
  scripts/deploy-production.sh --dry-run
  scripts/deploy-production.sh --build-only --output-dir /tmp/moonbook-release
  scripts/deploy-production.sh \
    --confirm-target root@101.32.210.229:/opt/moonbook-app

The script never uploads or prints the production .env file and never removes
Docker volumes, databases, R2 objects, or old images.
EOF
}

die() {
  printf 'deploy-production: %s\n' "$*" >&2
  exit 2
}

require_value() {
  local option="$1" value="${2:-}"
  [[ -n "$value" && "$value" != --* ]] || die "$option requires a value"
}

while (($#)); do
  case "$1" in
    --dry-run)
      [[ "$mode" == publish ]] || die "--dry-run cannot be combined with --build-only"
      mode="dry-run"
      shift
      ;;
    --build-only)
      [[ "$mode" == publish ]] || die "--build-only cannot be combined with --dry-run"
      mode="build-only"
      shift
      ;;
    --host)
      require_value "$1" "${2:-}"
      remote_host="$2"
      shift 2
      ;;
    --remote-dir)
      require_value "$1" "${2:-}"
      remote_dir="$2"
      shift 2
      ;;
    --platform)
      require_value "$1" "${2:-}"
      platform="$2"
      shift 2
      ;;
    --output-dir)
      require_value "$1" "${2:-}"
      output_dir="$2"
      shift 2
      ;;
    --confirm-target)
      require_value "$1" "${2:-}"
      confirmed_target="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *) die "unknown argument: $1" ;;
  esac
done

[[ "$remote_host" =~ ^[A-Za-z0-9._-]+@[A-Za-z0-9._:-]+$ ]] || die "invalid SSH target: $remote_host"
[[ "$remote_host" != -* ]] || die "invalid SSH target: $remote_host"
[[ "$remote_dir" == /* && "$remote_dir" != *$'\n'* ]] || die "remote directory must be an absolute path"
[[ "$platform" =~ ^[a-z0-9]+/[a-z0-9_]+$ ]] || die "invalid Docker platform: $platform"
if [[ -n "$output_dir" ]]; then
  [[ "$output_dir" == /* && "$output_dir" != *$'\n'* ]] || die "output directory must be an absolute path"
  case "$output_dir/" in
    "$root_dir/"*) die "output directory must be outside the repository" ;;
  esac
fi
if [[ "$mode" == publish && "$confirmed_target" != "$remote_host:$remote_dir" ]]; then
  die "refusing production deployment; pass --confirm-target '$remote_host:$remote_dir'"
fi

command -v git >/dev/null 2>&1 || die "missing required command: git"
repo_root="$(git -C "$root_dir" rev-parse --show-toplevel 2>/dev/null)" || die "not inside a Git repository"
[[ "$repo_root" == "$root_dir" ]] || die "script must run from the Moonbook repository"
[[ -z "$(git -C "$root_dir" status --porcelain)" ]] || die "Git worktree is dirty; commit or remove local changes before building"
commit="$(git -C "$root_dir" rev-parse HEAD)"
short_sha="$(git -C "$root_dir" rev-parse --short=12 HEAD)"
[[ "$commit" =~ ^[0-9a-f]{40}$ && "$short_sha" =~ ^[0-9a-f]{7,12}$ ]] || die "Git commit is invalid"

server_image="moonbook/server:$short_sha"
web_image="moonbook/web:$short_sha"
reader_image="moonbook/reader-ui:$short_sha"

print_plan() {
  printf 'mode=%s\n' "$mode"
  printf 'commit=%s\n' "$commit"
  printf 'platform=%s\n' "$platform"
  printf 'target=%s:%s\n' "$remote_host" "$remote_dir"
  printf 'images=%s,%s,%s\n' "$server_image" "$web_image" "$reader_image"
  printf 'stages=build,package,upload,verify,load,prepare,migrate,recreate,health\n'
}

if [[ "$mode" == dry-run ]]; then
  print_plan
  printf 'dry-run complete; Docker, SSH and SCP were not invoked\n'
  exit 0
fi

for command in docker gzip; do
  command -v "$command" >/dev/null 2>&1 || die "missing required command: $command"
done
docker buildx version >/dev/null 2>&1 || die "Docker Buildx is unavailable"
if [[ "$mode" == publish ]]; then
  for command in ssh scp; do
    command -v "$command" >/dev/null 2>&1 || die "missing required command: $command"
  done
fi

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    die "missing SHA-256 command: sha256sum or shasum"
  fi
}

if [[ -z "$output_dir" ]]; then
  output_dir="$(mktemp -d "${TMPDIR:-/tmp}/moonbook-release.$short_sha.XXXXXX")"
else
  mkdir -p "$output_dir"
  output_dir="$(cd "$output_dir" && pwd -P)"
fi
chmod 700 "$output_dir"

printf '[1/8] building %s\n' "$server_image"
docker buildx build --platform "$platform" --load -t "$server_image" -f "$root_dir/server/Dockerfile" "$root_dir/server"
printf '[2/8] building %s\n' "$web_image"
docker buildx build --platform "$platform" --load -t "$web_image" -f "$root_dir/web/Dockerfile" "$root_dir/web"
printf '[3/8] building %s\n' "$reader_image"
docker buildx build --platform "$platform" --load -t "$reader_image" \
  --build-arg VITE_READER_API_BASE=/prod-api \
  -f "$root_dir/deploy/compose/reader-ui.Dockerfile" "$root_dir"

archive_name="moonbook-images-$short_sha.tar.gz"
archive_path="$output_dir/$archive_name"
checksum_name="$archive_name.sha256"
checksum_path="$output_dir/$checksum_name"
manifest_path="$output_dir/release-$short_sha.txt"

printf '[4/8] packaging images\n'
docker save "$server_image" "$web_image" "$reader_image" | gzip -c >"$archive_path"
archive_sha="$(sha256_file "$archive_path")"
printf '%s  %s\n' "$archive_sha" "$archive_name" >"$checksum_path"
{
  printf 'commit=%s\n' "$commit"
  printf 'platform=%s\n' "$platform"
  printf 'server_image=%s\n' "$server_image"
  printf 'server_image_id=%s\n' "$(docker image inspect --format '{{.Id}}' "$server_image")"
  printf 'web_image=%s\n' "$web_image"
  printf 'web_image_id=%s\n' "$(docker image inspect --format '{{.Id}}' "$web_image")"
  printf 'reader_image=%s\n' "$reader_image"
  printf 'reader_image_id=%s\n' "$(docker image inspect --format '{{.Id}}' "$reader_image")"
  printf 'archive=%s\n' "$archive_name"
  printf 'archive_sha256=%s\n' "$archive_sha"
} >"$manifest_path"
chmod 600 "$archive_path" "$checksum_path" "$manifest_path"

if [[ "$mode" == build-only ]]; then
  printf 'build-only complete: %s\n' "$output_dir"
  printf 'archive_sha256=%s\n' "$archive_sha"
  exit 0
fi

ssh_options=(-o BatchMode=yes -o ConnectTimeout=10 -o ServerAliveInterval=15 -o ServerAliveCountMax=3)
remote_release_dir="/tmp/moonbook-release-$short_sha-$$"

remote_stage() {
  local stage="$1"
  shift
  ssh "${ssh_options[@]}" "$remote_host" bash -s -- "$stage" "$remote_dir" "$remote_release_dir" "$@" <<'REMOTE_SCRIPT'
set -euo pipefail
umask 077

stage="$1"
app_dir="$2"
release_dir="$3"
shift 3

[[ "$app_dir" == /* && "$release_dir" == /tmp/moonbook-release-* ]] || {
  echo "invalid remote deployment path" >&2
  exit 2
}

compose_file="$app_dir/compose.yml"
env_file="$app_dir/.env"
release_file="$app_dir/compose.release.yml"
previous_file="$app_dir/compose.release.previous.yml"

base_compose=(docker compose --project-directory "$app_dir" --env-file "$env_file" -f "$compose_file")
target_compose=("${base_compose[@]}" -f "$release_file")

write_override() {
  local destination="$1" server="$2" web="$3" reader="$4" temporary
  temporary="$destination.new.$$"
  {
    printf 'services:\n'
    printf '  migrate:\n    image: %s\n' "$server"
    printf '    environment:\n      MOONBOOK_APP_MASTER_KEY: ${MOONBOOK_APP_MASTER_KEY:?MOONBOOK_APP_MASTER_KEY is required}\n'
    printf '  server:\n    image: %s\n' "$server"
    printf '    environment:\n      MOONBOOK_APP_MASTER_KEY: ${MOONBOOK_APP_MASTER_KEY:?MOONBOOK_APP_MASTER_KEY is required}\n'
    printf '  web:\n    image: %s\n' "$web"
    printf '  reader-ui:\n    image: %s\n' "$reader"
    printf '  gateway:\n    image: %s\n' "$web"
  } >"$temporary"
  chmod 640 "$temporary"
  chown root:root "$temporary"
  mv -f "$temporary" "$destination"
}

case "$stage" in
  preflight)
    archive_bytes="$1"
    [[ -d "$app_dir" ]] || { echo "production directory not found: $app_dir" >&2; exit 2; }
    [[ -f "$compose_file" ]] || { echo "production compose file not found" >&2; exit 2; }
    [[ -s "$env_file" ]] || { echo "production environment file not found or empty" >&2; exit 2; }
    [[ "$(stat -c '%U:%G' "$env_file")" == root:root ]] || { echo "production environment file must be owned by root" >&2; exit 2; }
    case "$(stat -c '%a' "$env_file")" in 400|600) ;; *) echo "production environment file permissions must be 0400 or 0600" >&2; exit 2 ;; esac
    grep -Eq '^MOONBOOK_APP_MASTER_KEY=.+$' "$env_file" || { echo "MOONBOOK_APP_MASTER_KEY is missing from production environment" >&2; exit 2; }
    for command in docker gzip sha256sum curl; do
      command -v "$command" >/dev/null 2>&1 || { echo "missing remote command: $command" >&2; exit 2; }
    done
    compose_services="$("${base_compose[@]}" config --services)"
    for service in migrate server web reader-ui gateway; do
      grep -qx "$service" <<<"$compose_services" || { echo "production compose service is missing: $service" >&2; exit 2; }
    done
    available_kb="$(df -Pk "$app_dir" | awk 'NR==2 {print $4}')"
    required_kb=$(( (archive_bytes * 3 + 1023) / 1024 + 524288 ))
    ((available_kb >= required_kb)) || { echo "insufficient remote disk space for release" >&2; exit 2; }
    install -d -m 700 "$release_dir"
    ;;
  import)
    archive_name="$1"
    checksum_name="$2"
    [[ "$archive_name" =~ ^moonbook-images-[0-9a-f]{7,12}\.tar\.gz$ ]] || exit 2
    [[ "$checksum_name" == "$archive_name.sha256" ]] || exit 2
    cd "$release_dir"
    sha256sum -c "$checksum_name"
    gzip -dc "$archive_name" | docker load
    ;;
  prepare)
    server_image="$1"
    web_image="$2"
    reader_image="$3"
    [[ "$server_image" =~ ^moonbook/server:[0-9a-f]{7,12}$ ]] || exit 2
    [[ "$web_image" =~ ^moonbook/web:[0-9a-f]{7,12}$ ]] || exit 2
    [[ "$reader_image" =~ ^moonbook/reader-ui:[0-9a-f]{7,12}$ ]] || exit 2

    service_image() {
      local container_id
      container_id="$("${base_compose[@]}" ps -q "$1")"
      [[ -n "$container_id" ]] || { echo "service is not running: $1" >&2; exit 2; }
      docker inspect --format '{{.Config.Image}}' "$container_id"
    }
    old_server="$(service_image server)"
    old_web="$(service_image web)"
    old_reader="$(service_image reader-ui)"
    write_override "$previous_file" "$old_server" "$old_web" "$old_reader"
    write_override "$release_file" "$server_image" "$web_image" "$reader_image"

    "${target_compose[@]}" config --quiet
    printf 'previous_server_image=%s\n' "$old_server"
    printf 'previous_web_image=%s\n' "$old_web"
    printf 'previous_reader_image=%s\n' "$old_reader"
    ;;
  migrate)
    [[ -f "$release_file" ]] || { echo "release override not found" >&2; exit 2; }
    "${target_compose[@]}" run --rm --no-deps migrate
    ;;
  recreate)
    [[ -f "$release_file" ]] || { echo "release override not found" >&2; exit 2; }
    "${target_compose[@]}" up -d --no-build --force-recreate --wait --wait-timeout 240 server web reader-ui gateway
    ;;
  health)
    curl -fsS http://127.0.0.1:18080/gateway-health >/dev/null
    curl -fsS http://127.0.0.1:18080/api/health/ready >/dev/null
    curl -fsS http://127.0.0.1:18081/health >/dev/null
    ;;
  cleanup)
    rm -rf -- "$release_dir"
    ;;
  *) echo "unknown remote stage: $stage" >&2; exit 2 ;;
esac
REMOTE_SCRIPT
}

cleanup_remote() {
  local status=$?
  trap - EXIT INT TERM
  if ((remote_created)); then
    remote_stage cleanup >/dev/null 2>&1 || true
  fi
  exit "$status"
}
trap cleanup_remote EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

archive_bytes="$(wc -c <"$archive_path" | tr -d ' ')"
printf '[5/8] checking remote target\n'
remote_stage preflight "$archive_bytes"
remote_created=1

printf '[6/8] uploading and verifying release archive\n'
scp "${ssh_options[@]}" "$archive_path" "$checksum_path" "$manifest_path" "$remote_host:$remote_release_dir/"
remote_stage import "$archive_name" "$checksum_name"

printf '[7/8] preparing release override and running migration\n'
remote_stage prepare "$server_image" "$web_image" "$reader_image"
remote_stage migrate

printf '[8/8] replacing application containers and checking health\n'
remote_stage recreate
remote_stage health
printf 'production health checks passed\n'
remote_stage cleanup
remote_created=0
trap - EXIT INT TERM

printf 'deployment complete: commit=%s target=%s:%s\n' "$commit" "$remote_host" "$remote_dir"
printf 'artifacts=%s\n' "$output_dir"
printf 'rollback command (migration compatibility must be confirmed first):\n'
printf 'ssh %q %q\n' "$remote_host" \
  "cd '$remote_dir' && docker compose --project-directory '$remote_dir' --env-file '$remote_dir/.env' -f '$remote_dir/compose.yml' -f '$remote_dir/compose.release.previous.yml' up -d --no-build --force-recreate --wait server web reader-ui gateway"
