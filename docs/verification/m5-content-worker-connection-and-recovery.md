# M5 论坛连接检查与内容 Worker 重载恢复验收

验收日期：2026-08-21

## 结论

论坛来源管理端已提供脱敏连接检查。检查器在 DNS 解析和实际拨号两层拒绝私网、回环、链路本地、多播和未指定地址，生产只允许 HTTP/HTTPS 标准端口，关闭环境代理并限制同 hostname 重定向、总超时和响应体大小。Cookie 只在发请求前由 Secret 引用解析，响应不包含 URL、解析 IP、Cookie、正文或底层错误。

论坛导入、论坛发现、TXT 导入、章节清洗、章节摘要和作品画像 Worker 已纳入同一个串行生命周期管理器。重复或并发 Start 会先等待上一代全部退出；若停止超时，新一代拒绝启动，因此配置重载不会形成两套活动 Worker。

本报告关闭剩余收敛计划 Task 5。真实第三方论坛、真实 AI 服务和约 8 GB 数据副本不在本地自动化授权范围内，仍由后续 R3/R5 验收关闭。

## 安全与接口合同

- 新增 `POST /novel/crawl/sources/:id/check`，迁移 `00070_forum_source_connection_check.sql` 注册 API 和超级管理员策略。
- 默认总超时 10 秒、响应上限 64 KiB、最多三次同 hostname 重定向；跨 hostname 重定向直接返回 `REDIRECT_BLOCKED`。
- 稳定结果码为 `OK`、`AUTH_FAILED`、`REDIRECT_BLOCKED`、`TIMEOUT`、`RESPONSE_TOO_LARGE`、`DNS_FAILED`、`CONNECT_FAILED`、`HTTP_ERROR`、`TARGET_BLOCKED`、`SECRET_NOT_CONFIGURED`。
- 管理响应固定为 `ok`、`code`、`httpStatus`、`elapsedMs` 四个字段；前端只映射稳定中文结果，Long ID 仍由字符串路径工具拼接。
- 回环地址白名单只通过显式测试构造器注入；生产组合根使用无白名单默认构造器。

## 生命周期与恢复证据

- 组合根竞态测试覆盖四路并发替换，最大活动 Worker 集合为 1，所有代次均完成 Stop。
- Stop→Start 测试证明每次只创建一个新代次；不响应取消的旧 Worker 会触发超时并拒绝替换。
- 论坛导入、TXT、章节清洗、章节摘要和作品画像继续复用各模块已有的真实 PostgreSQL/MinIO 租约过期恢复集成测试：新 Worker 接管后 attempt 为 2，章节、对象、清洗结果、摘要和画像建议保持唯一。
- 论坛发现没有持久化业务 Job；其单例性和停止等待由同一组合根管理器测试覆盖，板块级 advisory lock 证据保持不变。

## 验证结果

- 本机 HTTP 替身覆盖请求头、成功、401/403、500、同/跨 hostname 重定向、超时、Content-Length/流式超限、DNS 失败、连接拒绝、SSRF 阻断和响应脱敏。
- `go test -race` 对连接检查、组合根和迁移包通过；`go vet ./...` 通过。
- PostgreSQL 17 隔离临时库验证空库 0→70、69→70、API/Casbin/序列和重复执行，重放应用数为 0；测试自动清理临时库。
- 管理前端 33 个 Node 合同测试、ESLint、生产构建和供应链检查通过。
- 仓库全量 `go test ./...` 仍受既有基线失败影响：Commerce 投影所有权规则、依赖运行中 8888 MCP 服务的客户端测试、GVA 模板相对路径、插件路由/Markdown 快照；本任务涉及包均通过，未放宽或跳过这些失败。

## 服务影响

- `migrate`：新增迁移 70，部署时必须先运行迁移任务。
- `server`：新增来源连接检查和内容 Worker 生命周期管理，迁移完成后必须重启。
- `web`：论坛来源页面新增连接检查操作，需要重新构建并发布静态资源。
- PostgreSQL 服务本身、Redis、MinIO、Reader UI 和 Gateway 无代码变化，无需单独重启。
