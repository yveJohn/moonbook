#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
reader_business_baseline="26743db"
reader_router_version="7.18.2"
mode="${1:---all}"

case "$mode" in
  --all|--reader-only|--integration-only) ;;
  *) echo "usage: scripts/verify-m3.sh [--all|--reader-only|--integration-only]" >&2; exit 2 ;;
esac

for command in git go npm; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "缺少必需命令: $command" >&2
    exit 2
  fi
done

cd "$repo_dir"

if [[ "$mode" == "--integration-only" ]]; then
  ./scripts/test-reader-integration.sh
  echo "M3 Reader/Commerce 真实依赖集成验收通过"
  exit 0
fi

echo "[1/6] 校验冻结 Reader 业务源码和安全依赖"
if ! git diff --quiet "$reader_business_baseline" -- reader-ui \
  ':(exclude)reader-ui/package.json' \
  ':(exclude)reader-ui/package-lock.json'; then
  echo "Reader 业务源码偏离冻结基线: $reader_business_baseline" >&2
  git diff --stat "$reader_business_baseline" -- reader-ui \
    ':(exclude)reader-ui/package.json' \
    ':(exclude)reader-ui/package-lock.json' >&2
  exit 1
fi
if [[ -n "$(git status --porcelain --untracked-files=all -- reader-ui \
  ':(exclude)reader-ui/package.json' \
  ':(exclude)reader-ui/package-lock.json')" ]]; then
  echo "Reader 业务源码工作树存在未提交变更" >&2
  git status --short --untracked-files=all -- reader-ui \
    ':(exclude)reader-ui/package.json' \
    ':(exclude)reader-ui/package-lock.json' >&2
  exit 1
fi
node -e '
  const fs = require("fs")
  const expected = process.argv[1]
  const manifest = JSON.parse(fs.readFileSync("reader-ui/package.json", "utf8"))
  const lock = JSON.parse(fs.readFileSync("reader-ui/package-lock.json", "utf8"))
  const packages = ["@react-router/node", "@react-router/serve", "@react-router/dev", "react-router-dom"]
  for (const name of packages) {
    const declared = manifest.dependencies?.[name] ?? manifest.devDependencies?.[name]
    const installed = lock.packages?.[`node_modules/${name}`]?.version
    if (declared !== expected || installed !== expected) {
      throw new Error(`${name} must be pinned to ${expected}; declared=${declared} lock=${installed}`)
    }
  }
' "$reader_router_version"

echo "[2/6] 运行 Reader/Commerce 后端单元与契约测试"
(
  cd server
  GOCACHE="${GOCACHE:-/tmp/moonbook-go-cache}" CGO_ENABLED=0 go test \
    ./internal/modules/reader/... \
    ./internal/modules/commerce/...
)

echo "[3/6] 运行真实 PostgreSQL/Redis/MinIO 集成测试"
if [[ "$mode" == "--all" ]]; then
  ./scripts/test-reader-integration.sh
else
  echo "由 integration 阶段独立执行"
fi

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

echo "M3 Reader 冻结业务源码与安全依赖兼容验收通过"
