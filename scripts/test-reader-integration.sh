#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir/server"

echo "运行 Moonbook Reader/Commerce 真实依赖集成测试"
required_vars=(
  MOONBOOK_READER_TEST_DSN
  MOONBOOK_READER_TEST_REDIS_ADDR
  MOONBOOK_READER_TEST_MINIO_ENDPOINT
  MOONBOOK_READER_TEST_MINIO_ACCESS_KEY
  MOONBOOK_READER_TEST_MINIO_SECRET_KEY
)
for name in "${required_vars[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    echo "缺少必需环境变量: $name" >&2
    exit 2
  fi
done

GOCACHE="${GOCACHE:-/tmp/moonbook-go-cache}" CGO_ENABLED=0 go test -tags=integration -count=1 -v \
  ./internal/modules/reader/... \
  ./internal/modules/commerce/...
