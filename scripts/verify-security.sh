#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
output_dir="${MOONBOOK_SECURITY_OUTPUT_DIR:-}"
[[ -n "$output_dir" && "$output_dir" == /* ]] || {
  echo "MOONBOOK_SECURITY_OUTPUT_DIR must be an absolute path" >&2
  exit 2
}
mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd -P)"

for command in docker git go jq npm pnpm; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing required command: $command" >&2; exit 2; }
done

short_commit="$(git -C "$root_dir" rev-parse --short=12 HEAD)"
server_image="moonbook/server:verify-$short_commit"
web_image="moonbook/web:verify-$short_commit"
reader_image="moonbook/reader-ui:verify-$short_commit"

"$root_dir/scripts/scan-secrets.sh"
(
  cd "$root_dir/server"
  GOCACHE="${GOCACHE:-/tmp/moonbook-go-cache}" CGO_ENABLED=0 go test \
    ./config ./middleware ./internal/platform/adminbootstrap ./internal/platform/apperror \
    ./internal/modules/reader/auth ./internal/modules/novel/crawlsource
)

docker build -t "$server_image" "$root_dir/server"
docker build -t "$web_image" "$root_dir/web"
docker build -t "$reader_image" -f "$root_dir/deploy/compose/reader-ui.Dockerfile" "$root_dir"

SERVER_IMAGE="$server_image" WEB_IMAGE="$web_image" READER_IMAGE="$reader_image" \
  "$root_dir/scripts/scan-supply-chain.sh" "$output_dir"

docker run --rm --entrypoint nginx \
  --add-host server:127.0.0.1 \
  --add-host web:127.0.0.1 \
  --add-host reader-ui:127.0.0.1 \
  -v "$root_dir/deploy/compose/gateway.conf:/etc/nginx/conf.d/default.conf:ro" \
  nginx:1.30.3-alpine3.23@sha256:0d3b80406a13a767339fbe2f41406d6c7da727ab89cf8fae399e81f780f814d1 -t

echo "security verification passed: behavior, secrets, locks, images, SBOMs, and gateway headers"
