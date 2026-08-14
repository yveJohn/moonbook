#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir/server"

echo "运行 Moonbook Reader/Commerce 真实依赖集成测试"
echo "未配置的依赖变量会导致对应测试 Skip；Skip 不构成验收通过。"
GOCACHE="${GOCACHE:-/tmp/moonbook-go-cache}" CGO_ENABLED=0 go test -tags=integration -count=1 -v \
  ./internal/modules/reader/auth \
  ./internal/modules/reader/public \
  ./internal/modules/reader/me \
  ./internal/modules/commerce/catalog
