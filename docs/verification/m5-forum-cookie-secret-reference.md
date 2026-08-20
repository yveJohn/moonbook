# M5 论坛 Cookie Secret 引用验收

验收日期：2026-08-21

## 结论

论坛来源不再接受、返回或写入 Cookie 明文。PostgreSQL 只保存受控环境变量引用；手动发现、自动发现和导入 Worker 均在请求前解析运行时 Secret，缺失或非法值以固定脱敏错误失败且不发出请求。

本报告关闭剩余收敛计划 Task 4，不代表真实第三方论坛沙箱、连接检查或内容 Worker 整体重启恢复已经完成。

## 数据与安全边界

- 新迁移 `00069_forum_cookie_secret_reference.sql` 增加 `cookie_secret_ref`，只允许 `MOONBOOK_FORUM_COOKIE_[A-Z0-9_]+`，长度上限为 128。
- 迁移先为来源生成 SHA-256 ID 指纹和处置状态审计，再受限清空非空 `cookie_text`；CHECK 约束永久拒绝新明文。
- 管理 API 只返回 `cookieSecretRef` 与 `cookieConfigured`，旧 `cookieText` 请求即使为空也明确拒绝。
- 运行时 Secret 缺失、空值、超长或包含换行时返回固定错误，不包含引用名或 Cookie 值。
- 请求日志同时掩码 `cookie` 与 `cookieText`；环境变量引用本身不作为 Secret 掩码。

## 自动化与真实验证

- Go 单元与合同测试覆盖引用格式、环境解析、非法值、明文请求拒绝、日志脱敏、Compose 环境注入、HTTP Cookie 请求头和未配置时零请求。
- 真实 PostgreSQL 17 覆盖空库 0→69、68→69 和重复执行；重放应用数为 0。
- 升级夹具验证来源行数不减少、明文字段清空、审计指纹长度为 64，且迁移后写入明文被数据库约束拒绝。
- 专属迁移后数据库上的来源 CRUD 和导入 Worker 集成测试通过；测试数据库在验收后删除。
- 管理前端 32 个 Node 合同测试、ESLint 和生产构建通过；页面没有 `cookieText`、明文 textarea、Long ID 数字转换或 `el-input-number`。
- 论坛 HTTP 本地替身验证导入与发现请求实际收到解析后的 Cookie；测试断言与失败输出不打印 Cookie。

## 服务影响

- `migrate`：新增迁移 69，部署时必须先执行迁移任务。
- `server`：论坛来源 API、发现 Worker、导入 Worker、组合根和日志脱敏变化，迁移完成并注入所需环境变量后必须重启。
- `web`：论坛来源编辑字段改为环境变量引用，需要重新构建并发布静态资源。
- PostgreSQL 服务本身、Redis、MinIO、Reader UI 和 Gateway 无代码变化，无需单独重启。
