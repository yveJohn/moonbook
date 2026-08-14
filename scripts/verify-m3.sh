#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
reader_tree_expected="ffbe7bb4c56792e94e160de86cc71bce0a6e0e5e"

for command in git go npm; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "缺少必需命令: $command" >&2
    exit 2
  fi
done

cd "$repo_dir"

echo "[1/6] 校验冻结 Reader Git 树"
reader_tree_actual="$(git rev-parse HEAD:reader-ui)"
if [[ "$reader_tree_actual" != "$reader_tree_expected" ]]; then
  echo "Reader tree 不匹配: expected=$reader_tree_expected actual=$reader_tree_actual" >&2
  exit 1
fi
if [[ -n "$(git status --porcelain --untracked-files=all -- reader-ui)" ]]; then
  echo "Reader 工作树存在未提交变更" >&2
  git status --short --untracked-files=all -- reader-ui >&2
  exit 1
fi

echo "[2/6] 运行 Reader/Commerce 后端单元与契约测试"
(
  cd server
  GOCACHE="${GOCACHE:-/tmp/moonbook-go-cache}" CGO_ENABLED=0 go test \
    ./internal/modules/reader/... \
    ./internal/modules/commerce/...
)

echo "[3/6] 运行真实 PostgreSQL/Redis/MinIO 集成测试"
./scripts/test-reader-integration.sh

echo "[4/6] 运行冻结 Reader 基线测试"
(
  cd reader-ui
  npm test
)

echo "[5/6] 运行树外 SSR/SEO 异常契约测试"
reader-ui/node_modules/.bin/vitest run --config tests/reader-ui-contract/vitest.config.ts

echo "[6/6] 构建冻结 Reader SSR 生产产物"
(
  cd reader-ui
  npm run build
)

echo "M3 Reader 零修改兼容验收通过"
