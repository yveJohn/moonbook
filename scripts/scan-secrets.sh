#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
image="${GITLEAKS_IMAGE:-zricethezav/gitleaks:v8.28.0}"

docker run --rm \
  -v "$root_dir:/repo:ro" \
  "$image" dir /repo \
  --config /repo/.gitleaks.toml \
  --gitleaks-ignore-path /repo/.gitleaksignore \
  --no-banner \
  --redact \
  --exit-code 1
