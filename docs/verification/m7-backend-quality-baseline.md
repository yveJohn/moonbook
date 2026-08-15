# M7 后端质量门验收

验证时间：2026-08-16
运行时代码基线：`7ec5d01c8a17531c212ae4701018f6dc304038af`

## 结论

模块边界整改后的后端格式、模块校验、`go vet`、普通测试和 Linux race 均通过。原基线发现的 15 处跨模块实现引用和 11 处跨域 SQL 已全部移除；依赖、SQL 所有权、表所有权和组合根静态门保持原样通过，没有增加放行规则。

Reader、Commerce、Novel 的真实 PostgreSQL/Redis/MinIO 回归通过。冻结 Reader tree 保持 `ffbe7bb4c56792e94e160de86cc71bce0a6e0e5e`，`make verify-m3` 完整通过 44 个测试文件、536 个冻结用例、6 个树外 SSR/SEO 用例和 Reader 生产构建。隔离 Compose 浏览器关键旅程也已闭环。

本结论只关闭模块边界整改的后端质量门，不关闭整个 M7：管理前端仍直接执行已确认恶意的 `vite-vue-path-map@1.0.2`，应用镜像漏洞、统一 CI 安全门和约 8 GB 完整副本演练仍未完成，生产发布继续 No-Go。

## 普通质量门

在 `server` 目录使用独立缓存和 `CGO_ENABLED=0` 执行：

```bash
gofmt -l internal/modules internal/platform \
  cmd/moonbook-admin cmd/moonbook-config \
  cmd/moonbook-legacy-migrate cmd/moonbook-migrate

go mod verify

CGO_ENABLED=0 GOCACHE=/tmp/moonbook-gocache go vet \
  ./internal/... ./config ./core/... ./initialize/... ./middleware \
  ./cmd/moonbook-admin ./cmd/moonbook-config \
  ./cmd/moonbook-legacy-migrate ./cmd/moonbook-migrate

CGO_ENABLED=0 GOCACHE=/tmp/moonbook-gocache go test \
  ./internal/... ./config ./core/... ./initialize/... ./middleware \
  ./cmd/moonbook-admin ./cmd/moonbook-config \
  ./cmd/moonbook-legacy-migrate ./cmd/moonbook-migrate
```

结果：`gofmt -l` 无输出，`go mod verify` 返回 `all modules verified`，`go vet` 和普通测试退出码均为 0。需要本机回环监听的 HTTP fixture 在允许回环监听后同范围通过。

## Linux race

本机 macOS ARM cgo 路径仍受上游 `go-m1cpu` 初始化问题影响，因此使用已有官方 `golang:1.24.2-bookworm` Linux 镜像、只读源码和只读模块缓存执行 CI 完整后端范围：

```bash
go test -race \
  ./internal/... ./config ./core/... ./initialize/... ./middleware \
  ./cmd/moonbook-admin ./cmd/moonbook-config \
  ./cmd/moonbook-legacy-migrate ./cmd/moonbook-migrate
```

结果：退出码 0。模块静态门、Reader/Commerce/Novel、迁移、事务和全部命令包均通过；并发重点包 `reader/invite`、`reader/me`、`commerce/payment`、`novel/provider`、`platform/transaction` 同时包含在该范围内。

## 真实依赖与 M3

- Reader/Commerce 四个交易相关包的真实依赖集成测试连续 10 轮通过；模拟充值、支付回调和人工补单的统一锁序未再出现 PostgreSQL 死锁。
- Reader/Commerce 全量真实 PostgreSQL/Redis/MinIO 脚本通过；脚本使用 `-p=1` 隔离共享验收数据库上的包级 fixture，包内并发测试保持启用。
- `make verify-m3` 完整通过且无 Skip：冻结 Reader tree 校验、Go 单元/契约、真实依赖、536 个冻结用例、6 个树外 SSR/SEO 用例和 Reader 生产构建全部成功。
- 隔离验收库执行 Reader 账号搜索投影修复后为 `reader_accounts=237`、`commerce_reader_search_projection=237`，`id/username/nickname/status` 双向差异为 0。

## 浏览器关键旅程

隔离 `moonbook_browser` 栈使用 PostgreSQL `25488`、Redis `26488`、MinIO `29488` 和 Reader `18489`。Playwright 已验证：

- 匿名书库与详情、匿名阅读登录跳转；
- 邀请码注册后回跳第一章，读取真实 MinIO 正文；
- 加入书架、两章切换、第二章历史持久化；
- 书架显示第二章，“继续阅读”恢复第二章；
- 退出后旧 Token 请求 `/reader/me/history` 返回业务 `401`，再次访问书架跳转登录；
- 全过程控制台错误为 0。

冻结 Reader 的历史上报语义是在离开当前章节或产生进度动作时保存当前章节；因此从第一章首次进入第二章只保存第一章，再从第二章切回第一章后第二章历史才会持久化。该行为与冻结源码一致，不是后端契约回归。

临时浏览器书籍、两章正文对象、账号、会话、邀请码、邀请关系、钱包和活动记录已从隔离栈清理，目标书籍与账号计数均为 0。

## 可信前端边界

没有执行管理端 `pnpm install` 或生产构建。静态检查确认：

- `web/package.json` 仍直接依赖 `vite-vue-path-map@^1.0.2`；
- `web/pnpm-lock.yaml` 仍固定解析为恶意版本 `1.0.2`；
- `web/vite.config.js` 仍导入并调用该插件；
- 现有 `web/dist` 与包实现仍命中 `generateBundle`、`document.open()`、`document.write()` 和 `pathInfo` 注入特征。

这些结果只证明安全阻断仍然存在，旧产物不得作为可信构建证据。

## 剩余阻断

1. 删除恶意管理端依赖、替换为可审计的本地路径映射实现并重新生成锁文件与镜像；
2. 修复 Server、Web、Reader 镜像的 high/critical 漏洞并补齐 SBOM、许可证和安全扫描门；
3. 在固定候选制品上执行约 8 GB 旧库副本的 M6 全量迁移、耗时、财务和对象一致性演练；
4. 把 M3 真实依赖、浏览器、迁移、备份恢复和供应链检查纳入统一 CI 发布门。
